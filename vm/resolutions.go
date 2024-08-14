// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/vilmo"
)

var (
	_ chain.VM                           = (*VM)(nil)
	_ block.ChainVM                      = (*VM)(nil)
	_ block.BuildBlockWithContextChainVM = (*VM)(nil)
)

func (vm *VM) ChainID() ids.ID {
	return vm.snowCtx.ChainID
}

func (vm *VM) NetworkID() uint32 {
	return vm.snowCtx.NetworkID
}

func (vm *VM) SubnetID() ids.ID {
	return vm.snowCtx.SubnetID
}

func (vm *VM) ValidatorState() validators.State {
	return vm.snowCtx.ValidatorState
}

func (vm *VM) Registry() (chain.ActionRegistry, chain.AuthRegistry) {
	return vm.actionRegistry, vm.authRegistry
}

func (vm *VM) Tracer() trace.Tracer {
	return vm.tracer
}

func (vm *VM) Logger() logging.Logger {
	return vm.snowCtx.Log
}

func (vm *VM) Rules(t int64) chain.Rules {
	return vm.c.Rules(t)
}

func (vm *VM) LastAcceptedBlock() *chain.StatelessBlock {
	return vm.lastAccepted
}

func (vm *VM) IsBootstrapped() bool {
	return vm.bootstrapped.Get()
}

func (vm *VM) State() *vilmo.Vilmo {
	return vm.stateDB
}

func (vm *VM) Mempool() chain.Mempool {
	return vm.mempool
}

func (vm *VM) IsRepeatTx(ctx context.Context, txs []*chain.Transaction, marker set.Bits) set.Bits {
	_, span := vm.tracer.Start(ctx, "VM.IsRepeatTx")
	defer span.End()

	return vm.seenTxs.Contains(txs, marker, false)
}

func (vm *VM) IsRepeatChunk(ctx context.Context, certs []*chain.ChunkCertificate, marker set.Bits) set.Bits {
	_, span := vm.tracer.Start(ctx, "VM.IsRepeatChunk")
	defer span.End()

	return vm.seenChunks.Contains(certs, marker, false)
}

func (vm *VM) IsSeenChunk(ctx context.Context, chunkID ids.ID) bool {
	return vm.seenChunks.HasID(chunkID)
}

func (vm *VM) Verified(ctx context.Context, b *chain.StatelessBlock) {
	_, span := vm.tracer.Start(ctx, "VM.Verified")
	defer span.End()

	vm.verifiedL.Lock()
	vm.verifiedBlocks[b.ID()] = b
	vm.verifiedL.Unlock()
	vm.parsedBlocks.Evict(b.ID())

	// We opt to not remove chunks [b.AvailableChunks] from [cm] here because
	// we may build on a different parent and we want to maximize the probability
	// any cert gets included. If this is not the case, the cert repeat inclusion check
	// is fast.
}

func (vm *VM) processExecutedChunks() {
	// Always close [acceptorDone] or we may block shutdown.
	defer func() {
		close(vm.executorDone)
		vm.snowCtx.Log.Info("executor queue shutdown")
	}()

	// The VM closes [executedQueue] during shutdown. We wait for all enqueued blocks
	// to be processed before returning as a guarantee to listeners (which may
	// persist indexed state) instead of just exiting as soon as `vm.stop` is
	// closed.
	for ew := range vm.executedQueue {
		vm.metrics.executedProcessingBacklog.Dec()
		if ew.Chunk != nil {
			vm.processExecutedChunk(ew.Block, ew.Chunk, ew.Results, ew.InvalidTxs)
			vm.snowCtx.Log.Debug(
				"chunk async executed",
				zap.Uint64("blk", ew.Block.Height),
				zap.Stringer("chunkID", ew.Chunk.Chunk),
			)
			continue
		}
		vm.processExecutedBlock(ew.Block)
		vm.snowCtx.Log.Debug(
			"block async executed",
			zap.Uint64("blk", ew.Block.Height),
		)
	}
}

func (vm *VM) ExecutedChunk(ctx context.Context, blk *chain.StatefulBlock, chunk *chain.FilteredChunk, results []*chain.Result, invalidTxs []ids.ID) {
	_, span := vm.tracer.Start(ctx, "VM.ExecutedChunk")
	defer span.End()

	// Mark all txs as seen (prevent replay in subsequent blocks)
	//
	// We do this before Accept to avoid maintaining a set of diffs
	// that we need to check for repeats on top of this.
	vm.seenTxs.Add(chunk.Txs)

	// Add chunk to backlog for async processing
	vm.metrics.executedProcessingBacklog.Inc()
	vm.executedQueue <- &executedWrapper{blk, chunk, results, invalidTxs}

	// Record units processed
	chunkUnits := chain.Dimensions{}
	for _, r := range results {
		nextUnits, err := chain.Add(chunkUnits, r.Consumed)
		if err != nil {
			vm.Fatal("unable to add executed units", zap.Error(err))
		}
		chunkUnits = nextUnits
	}
	vm.metrics.unitsExecutedBandwidth.Add(float64(chunkUnits[chain.Bandwidth]))
	vm.metrics.unitsExecutedCompute.Add(float64(chunkUnits[chain.Compute]))
	vm.metrics.unitsExecutedRead.Add(float64(chunkUnits[chain.StorageRead]))
	vm.metrics.unitsExecutedAllocate.Add(float64(chunkUnits[chain.StorageAllocate]))
	vm.metrics.unitsExecutedWrite.Add(float64(chunkUnits[chain.StorageWrite]))
}

func (vm *VM) ExecutedBlock(ctx context.Context, blk *chain.StatefulBlock) {
	_, span := vm.tracer.Start(ctx, "VM.ExecutedBlock")
	defer span.End()

	// We interleave results with chunks to ensure things are processed in the write order (if processed independently, we might
	// process a block execution before a chunk).
	vm.metrics.executedProcessingBacklog.Inc()
	vm.executedQueue <- &executedWrapper{Block: blk}
}

func (vm *VM) processExecutedBlock(blk *chain.StatefulBlock) {
	start := time.Now()
	defer func() {
		vm.metrics.executedBlockProcess.Observe(float64(time.Since(start)))
	}()

	// Clear authorization results
	vm.metrics.uselessChunkAuth.Add(float64(len(vm.cm.auth.SetMin(blk.Timestamp))))

	// Update timestamp in mempool
	//
	// We wait to update the min until here because we want to allow all execution
	// to complete and remove valid txs first.
	ctx := context.TODO()
	t := blk.Timestamp
	vm.metrics.mempoolExpired.Add(float64(len(vm.mempool.SetMinTimestamp(ctx, t))))
	vm.metrics.mempoolLen.Set(float64(vm.mempool.Len(ctx)))
	vm.metrics.mempoolSize.Set(float64(vm.mempool.Size(ctx)))

	// We need to wait until we may not try to verify the signature of a tx again.
	vm.rpcAuthorizedTxs.SetMin(t)

	// Must clear accepted txs before [SetMinTx] or else we will errnoueously
	// send [ErrExpired] messages.
	if err := vm.webSocketServer.SetMinTx(t); err != nil {
		vm.Fatal("unable to set min tx in websocket server", zap.Error(err))
	}
}

func (vm *VM) processExecutedChunk(
	blk *chain.StatefulBlock,
	chunk *chain.FilteredChunk,
	results []*chain.Result,
	invalidTxs []ids.ID,
) {
	start := time.Now()
	defer func() {
		vm.metrics.executedChunkProcess.Observe(float64(time.Since(start)))
	}()

	// Remove any executed transactions
	ctx := context.TODO()
	vm.mempool.Remove(ctx, chunk.Txs)

	// Sign and store any warp messages (regardless if validator now, may become one)
	for i, tx := range chunk.Txs { // filtered chunks only have valid txs
		// Only cache auth for accepted blocks to prevent cache manipulation from RPC submissions
		vm.cacheAuth(tx.Auth)

		result := results[i]
		if result.WarpMessage == nil {
			continue
		}
		start := time.Now()
		signature, err := vm.snowCtx.WarpSigner.Sign(result.WarpMessage)
		if err != nil {
			vm.Fatal("unable to sign warp message", zap.Error(err))
		}
		if err := vm.StoreWarpSignature(tx.ID(), vm.snowCtx.PublicKey, signature); err != nil {
			vm.Fatal("unable to store warp signature", zap.Error(err))
		}
		vm.snowCtx.Log.Info(
			"signed and stored warp message signature",
			zap.Stringer("txID", tx.ID()),
			zap.Duration("t", time.Since(start)),
		)

		// Kickoff job to fetch signatures from other validators in the
		// background
		//
		// We pass bytes here so that signatures returned from validators can be
		// verified before they are persisted.
		vm.warpManager.GatherSignatures(context.TODO(), tx.ID(), result.WarpMessage.Bytes())
	}

	// Send notifications as soon as transactions are executed
	if err := vm.webSocketServer.ExecuteChunk(blk.Height, chunk, results, invalidTxs); err != nil {
		vm.Fatal("unable to execute chunk in websocket server", zap.Error(err))
	}
}

func (vm *VM) Rejected(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Rejected")
	defer span.End()

	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()

	vm.cm.RestoreChunkCertificates(ctx, b.AvailableChunks)

	if err := vm.c.Rejected(ctx, b); err != nil {
		vm.Fatal("rejected processing failed", zap.Error(err))
	}

	// Ensure children of block are cleared, they may never be
	// verified
	vm.snowCtx.Log.Info("rejected block", zap.Stringer("id", b.ID()), zap.Uint64("height", b.Height()))
}

func (vm *VM) processAcceptedBlock(b *chain.StatelessBlock) {
	start := time.Now()
	defer func() {
		vm.metrics.blockProcess.Observe(float64(time.Since(start)))
	}()

	// // We skip blocks that were not processed because metadata required to
	// // process blocks opaquely (like looking at results) is not populated.
	// //
	// // We don't need to worry about dangling messages in listeners because we
	// // don't allow subscription until the node is healthy.
	// if !b.Processed() {
	// 	vm.snowCtx.Log.Info("skipping unprocessed block", zap.Uint64("height", b.Hght))
	// 	return
	// }

	// Update controller
	//
	// TODO: pass all chunk info here
	if err := vm.c.Accepted(context.TODO(), b); err != nil {
		vm.Fatal("accepted processing failed", zap.Error(err))
	}

	// Send notifications as soon as transactions are executed
	if err := vm.webSocketServer.AcceptBlock(b); err != nil {
		vm.Fatal("unable to accept block in websocket server", zap.Error(err))
	}
}

func (vm *VM) processAcceptedBlocks() {
	// Always close [acceptorDone] or we may block shutdown.
	defer func() {
		close(vm.acceptorDone)
		vm.snowCtx.Log.Info("acceptor queue shutdown")
	}()

	// The VM closes [acceptedQueue] during shutdown. We wait for all enqueued blocks
	// to be processed before returning as a guarantee to listeners (which may
	// persist indexed state) instead of just exiting as soon as `vm.stop` is
	// closed.
	for aw := range vm.acceptedQueue {
		// Commit filtered chunks
		g, _ := errgroup.WithContext(context.TODO())
		for _, fc := range aw.FilteredChunks {
			tfc := fc
			g.Go(func() error {
				return vm.StoreFilteredChunk(tfc)
			})
		}
		if err := g.Wait(); err != nil {
			vm.Fatal("unable to store filtered chunk", zap.Error(err))
		}

		// Process block
		vm.processAcceptedBlock(aw.Block)
		vm.snowCtx.Log.Info(
			"block async accepted",
			zap.Stringer("blkID", aw.Block.ID()),
			zap.Uint64("height", aw.Block.Height()),
		)

		// Delete old blocks and chunks
		if err := vm.PruneBlockAndChunks(aw.Block.Height()); err != nil {
			vm.Fatal("unable to prune block and chunks", zap.Error(err))
		}
	}
}

func (vm *VM) Accepted(ctx context.Context, b *chain.StatelessBlock, chunks []*chain.FilteredChunk) {
	ctx, span := vm.tracer.Start(ctx, "VM.Accepted")
	defer span.End()

	// Update accepted block on-disk and caches
	if err := vm.UpdateLastAccepted(b); err != nil {
		vm.Fatal("unable to update last accepted", zap.Error(err))
	}

	// Cleanup expired chunks we are tracking and chunk certificates
	vm.cm.SetBuildableMin(ctx, b.StatefulBlock.Timestamp) // clear unnecessary certs

	// Remove from verified caches
	//
	// We do this after setting [lastAccepted] to avoid
	// a race where the block isn't accessible.
	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()

	// Update issued transaction tracker once we know we can't resubmit the same transaction
	blkTime := b.StatefulBlock.Timestamp
	evictedIssued := vm.issuedTxs.SetMin(blkTime)
	vm.Logger().Debug("txs evicted from issued", zap.Int("len", len(evictedIssued)))

	// Update replay protection heap
	//
	// Transactions are added to [seen] with their [expiry], so we don't need to
	// transform [blkTime] when calling [SetMin] here.
	vm.Logger().Debug("txs evicted from seen", zap.Int("len", len(vm.seenTxs.SetMin(blkTime))))
	vm.Logger().Debug("chunks evicted from seen", zap.Int("len", len(vm.seenChunks.SetMin(blkTime))))
	vm.seenChunks.Add(b.AvailableChunks)

	// Verify if emap is now sufficient (we need a consecutive run of blocks with
	// timestamps of at least [ValidityWindow] for this to occur).
	if !vm.isReady() {
		select {
		case <-vm.seenValidityWindow:
			// We could not be ready but seen a window of transactions if the state
			// to sync is large (takes longer to fetch than [ValidityWindow]).
		default:
			// The value of [vm.startSeenTime] can only be negative if we are
			// performing state sync.
			if vm.startSeenTime < 0 {
				vm.startSeenTime = blkTime
			}
			r := vm.Rules(blkTime)
			if blkTime-vm.startSeenTime > r.GetValidityWindow() {
				vm.seenValidityWindowOnce.Do(func() {
					close(vm.seenValidityWindow)
				})
			}
		}
	}

	// Enqueue block for processing
	vm.acceptedQueue <- &acceptedWrapper{b, chunks}
}

func (vm *VM) CacheValidators(ctx context.Context, height uint64) {
	vm.proposerMonitor.Fetch(ctx, height)
}

func (vm *VM) AddressPartition(ctx context.Context, epoch uint64, height uint64, addr codec.Address, partition uint8) (ids.NodeID, error) {
	return vm.proposerMonitor.AddressPartition(ctx, epoch, height, addr, partition)
}

// used for txs from Anchor
func (vm *VM) AddressPartitionByNamespace(ctx context.Context, epoch uint64, height uint64, ns []byte, partition uint8) (ids.NodeID, error) {
	return vm.proposerMonitor.AddressPartitionByNamespace(ctx, epoch, height, ns, partition)
}

func (vm *VM) IsValidator(ctx context.Context, height uint64, nid ids.NodeID) (bool, error) {
	ok, _, _, err := vm.proposerMonitor.IsValidator(ctx, height, nid)
	return ok, err
}

func (vm *VM) GetWarpValidators(ctx context.Context, height uint64) ([]*warp.Validator, uint64, error) {
	return vm.proposerMonitor.GetWarpValidatorSet(ctx, height)
}

func (vm *VM) IterateValidators(
	ctx context.Context,
	height uint64,
	fn func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	return vm.proposerMonitor.IterateValidators(ctx, height, fn)
}

func (vm *VM) IterateCurrentValidators(
	ctx context.Context,
	fn func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	return vm.proposerMonitor.IterateCurrentValidators(ctx, fn)
}

func (vm *VM) GetAggregatePublicKey(ctx context.Context, height uint64, signers set.Bits, num, denom uint64) (*bls.PublicKey, error) {
	return vm.proposerMonitor.GetAggregatePublicKey(ctx, height, signers, num, denom)
}

func (vm *VM) IsValidHeight(ctx context.Context, height uint64) (bool, error) {
	return vm.proposerMonitor.IsValidHeight(ctx, height)
}

func (vm *VM) GatherSignatures(ctx context.Context, txID ids.ID, msg []byte) {
	vm.warpManager.GatherSignatures(ctx, txID, msg)
}

func (vm *VM) NodeID() ids.NodeID {
	return vm.snowCtx.NodeID
}

func (vm *VM) PreferredBlock(ctx context.Context) (*chain.StatelessBlock, error) {
	return vm.GetStatelessBlock(ctx, vm.preferred)
}

func (vm *VM) StopChan() chan struct{} {
	return vm.stop
}

func (vm *VM) EngineChan() chan<- common.Message {
	return vm.toEngine
}

func (vm *VM) StateManager() chain.StateManager {
	return vm.c.StateManager()
}

func (vm *VM) RecordWaitAuth(t time.Duration) {
	vm.metrics.waitAuth.Observe(float64(t))
}

func (vm *VM) RecordWaitExec(t time.Duration) {
	vm.metrics.waitExec.Observe(float64(t))
}

func (vm *VM) RecordWaitPrecheck(t time.Duration) {
	vm.metrics.waitPrecheck.Observe(float64(t))
}

func (vm *VM) RecordWaitCommit(t time.Duration) {
	vm.metrics.waitCommit.Observe(float64(t))
}

func (vm *VM) RecordStateChanges(c int) {
	vm.metrics.stateChanges.Observe(float64(c))
}

func (vm *VM) GetVerifyAuth() bool {
	return vm.config.GetVerifyAuth()
}

func (vm *VM) GetAuthExecutionCores() int {
	return vm.config.GetAuthExecutionCores()
}

// This must be non-nil or the VM won't be able to produce chunks
func (vm *VM) Beneficiary() codec.Address {
	return vm.config.GetBeneficiary()
}

func (vm *VM) RecordTxsGossiped(c int) {
	vm.metrics.txsGossiped.Add(float64(c))
}

func (vm *VM) RecordTxsReceived(c int) {
	vm.metrics.txsReceived.Add(float64(c))
}

func (vm *VM) RecordTxsIncluded(c int) {
	vm.metrics.txsIncluded.Add(float64(c))
}

func (vm *VM) RecordTxsInvalid(c int) {
	vm.metrics.txsInvalid.Add(float64(c))
}

func (vm *VM) GetTargetChunkBuildDuration() time.Duration {
	return vm.config.GetTargetChunkBuildDuration()
}

func (vm *VM) cacheAuth(auth chain.Auth) {
	bv, ok := vm.authEngine[auth.GetTypeID()]
	if !ok {
		return
	}
	bv.Cache(auth)
}

func (vm *VM) RecordBlockVerify(t time.Duration) {
	vm.metrics.blockVerify.Observe(float64(t))
}

func (vm *VM) RecordBlockAccept(t time.Duration) {
	vm.metrics.blockAccept.Observe(float64(t))
}

func (vm *VM) RecordBlockExecute(t time.Duration) {
	vm.metrics.blockExecute.Observe(float64(t))
}

func (vm *VM) RecordRemainingMempool(l int) {
	vm.metrics.remainingMempool.Add(float64(l))
}

func (vm *VM) UnitPrices(context.Context) (chain.Dimensions, error) {
	return vm.Rules(time.Now().UnixMilli()).GetUnitPrices(), nil
}

func (vm *VM) GetActionExecutionCores() int {
	return vm.config.GetActionExecutionCores()
}

func (vm *VM) GetExecutorRecorder() executor.Metrics {
	return vm.metrics.executorRecorder
}

func (vm *VM) StartCertStream(context.Context) {
	vm.cm.certs.StartStream()
}

func (vm *VM) StreamCert(ctx context.Context) (*chain.ChunkCertificate, bool) {
	return vm.cm.certs.Stream(ctx)
}

func (vm *VM) FinishCertStream(_ context.Context, certs []*chain.ChunkCertificate) {
	vm.cm.certs.FinishStream(certs)
}

func (vm *VM) RestoreChunkCertificates(ctx context.Context, certs []*chain.ChunkCertificate) {
	vm.cm.RestoreChunkCertificates(ctx, certs)
}

func (vm *VM) Engine() *chain.Engine {
	return vm.engine
}

func (vm *VM) HandleAnchorChunk(ctx context.Context, anchor *chain.Anchor, slot int64, txs []*chain.Transaction, priorityFeeReceiverAddr codec.Address) error {
	return vm.cm.HandleAnchorChunk(ctx, anchor, slot, txs, priorityFeeReceiverAddr)
}

func (vm *VM) IsIssuedTx(_ context.Context, tx *chain.Transaction) bool {
	return vm.issuedTxs.Has(tx)
}

func (vm *VM) IssueTx(_ context.Context, tx *chain.Transaction) {
	vm.issuedTxs.Add([]*chain.Transaction{tx})
}

func (vm *VM) Signer() *bls.PublicKey {
	return vm.snowCtx.PublicKey
}

func (vm *VM) Sign(msg *warp.UnsignedMessage) ([]byte, error) {
	return vm.snowCtx.WarpSigner.Sign(msg)
}

func (vm *VM) RequestChunks(block uint64, certs []*chain.ChunkCertificate, chunks chan *chain.Chunk) {
	vm.cm.RequestChunks(block, certs, chunks)
}

func (vm *VM) RecordEngineBacklog(c int) {
	vm.metrics.engineBacklog.Add(float64(c))
}

func (vm *VM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	bv, ok := vm.authEngine[authTypeID]
	if !ok {
		return nil, false
	}
	return bv.GetBatchVerifier(cores, count), ok
}

func (vm *VM) RecordExecutedChunks(c int) {
	vm.metrics.chunksExecuted.Add(float64(c))
}

func (vm *VM) RecordWaitRepeat(t time.Duration) {
	vm.metrics.waitRepeat.Observe(float64(t))
}

func (vm *VM) GetAuthRPCCores() int {
	return vm.config.GetAuthRPCCores()
}

func (vm *VM) GetAuthRPCBacklog() int {
	return vm.config.GetAuthRPCBacklog()
}

func (vm *VM) RecordRPCTxBacklog(c int64) {
	vm.metrics.rpcTxBacklog.Set(float64(c))
}

func (vm *VM) AddRPCAuthorized(tx *chain.Transaction) {
	vm.rpcAuthorizedTxs.Add([]*chain.Transaction{tx})
}

func (vm *VM) IsRPCAuthorized(txID ids.ID) bool {
	return vm.rpcAuthorizedTxs.HasID(txID)
}

func (vm *VM) RecordRPCAuthorizedTx() {
	vm.metrics.txRPCAuthorized.Inc()
}

func (vm *VM) RecordBlockVerifyFail() {
	vm.metrics.blockVerifyFailed.Inc()
}

func (vm *VM) RecordWebsocketConnection(c int) {
	vm.metrics.websocketConnections.Add(float64(c))
}

func (vm *VM) RecordChunkBuildTxDropped() {
	vm.metrics.chunkBuildTxsDropped.Inc()
}

func (vm *VM) RecordRPCTxInvalid() {
	vm.metrics.rpcTxInvalid.Inc()
}

func (vm *VM) RecordBlockBuildCertDropped() {
	vm.metrics.blockBuildCertsDropped.Inc()
}

func (vm *VM) RecordAcceptedEpoch(e uint64) {
	vm.metrics.lastAcceptedEpoch.Set(float64(e))
}

func (vm *VM) RecordExecutedEpoch(e uint64) {
	vm.metrics.lastExecutedEpoch.Set(float64(e))
}

func (vm *VM) GetAuthResult(chunkID ids.ID) bool {
	// TODO: clean up this invocation
	return vm.cm.auth.Wait(chunkID)
}

func (vm *VM) RecordWaitQueue(t time.Duration) {
	vm.metrics.waitQueue.Observe(float64(t))
}

func (vm *VM) GetPrecheckCores() int {
	return vm.config.GetPrecheckCores()
}

func (vm *VM) RecordVilmoBatchInit(t time.Duration) {
	vm.metrics.appendDBBatchInit.Observe(float64(t))
}

func (vm *VM) RecordVilmoBatchInitBytes(b int64) {
	vm.metrics.appendDBBatchInitBytes.Observe(float64(b))
}

func (vm *VM) RecordVilmoBatchesRewritten() {
	vm.metrics.appendDBBatchesRewritten.Inc()
}

func (vm *VM) RecordVilmoBatchPrepare(t time.Duration) {
	vm.metrics.appendDBBatchPrepare.Observe(float64(t))
}

func (vm *VM) RecordTStateIterate(t time.Duration) {
	vm.metrics.tstateIterate.Observe(float64(t))
}

func (vm *VM) RecordVilmoBatchWrite(t time.Duration) {
	vm.metrics.appendDBBatchWrite.Observe(float64(t))
}

func (vm *VM) ProposerLookUp(ctx context.Context, height uint64, pChainHeight uint64, maxWindows int) ids.NodeID {
	return vm.proposerMonitor.ProposerLookUP(ctx, height, pChainHeight, maxWindows)
}
