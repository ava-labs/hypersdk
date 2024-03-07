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
	"github.com/ava-labs/avalanchego/x/merkledb"
	"golang.org/x/sync/errgroup"

	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/executor"
)

const diskConcurrency = 8

var (
	_ chain.VM                           = (*VM)(nil)
	_ block.ChainVM                      = (*VM)(nil)
	_ block.StateSyncableVM              = (*VM)(nil)
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

func (vm *VM) State() (merkledb.MerkleDB, error) {
	// As soon as synced (before ready), we can safely request data from the db.
	if !vm.StateReady() {
		return nil, ErrStateMissing
	}
	return vm.stateDB, nil
}

// TODO: correctly handle engine.Exeucte during state sync
func (vm *VM) ForceState() merkledb.MerkleDB {
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

func (vm *VM) Verified(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Verified")
	defer span.End()

	vm.verifiedL.Lock()
	vm.verifiedBlocks[b.ID()] = b
	vm.verifiedL.Unlock()
	vm.parsedBlocks.Evict(b.ID())
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
		vm.processExecutedChunk(ew.Block, ew.Chunk, ew.Results)
		vm.snowCtx.Log.Debug(
			"chunk async executed",
			zap.Uint64("blk", ew.Block),
			zap.Stringer("chunkID", ew.Chunk.Chunk),
		)
	}
}

func (vm *VM) Executed(ctx context.Context, blk uint64, chunk *chain.FilteredChunk, results []*chain.Result) {
	ctx, span := vm.tracer.Start(ctx, "VM.Executed")
	defer span.End()

	vm.executedQueue <- &executedWrapper{blk, chunk, results}
}

func (vm *VM) processExecutedChunk(blk uint64, chunk *chain.FilteredChunk, results []*chain.Result) {
	// Remove any executed transactions
	vm.mempool.Remove(context.TODO(), chunk.Txs)

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
	if err := vm.webSocketServer.ExecuteChunk(blk, chunk, results); err != nil {
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

	// Must clear accepted txs before [SetMinTx] or else we will errnoueously
	// send [ErrExpired] messages.
	if err := vm.webSocketServer.SetMinTx(b.StatefulBlock.Timestamp); err != nil {
		vm.Fatal("unable to set min tx in websocket server", zap.Error(err))
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
		g, _ := errgroup.WithContext(context.Background())
		g.SetLimit(diskConcurrency)
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
	vm.cm.SetMin(ctx, b.StatefulBlock.Timestamp) // clear unnecessary certs

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
	evictedTxs := vm.seenTxs.SetMin(blkTime)
	vm.Logger().Debug("txs evicted from seen", zap.Int("len", len(evictedTxs)))
	for _, fc := range chunks {
		// Mark all valid txs as seen
		vm.seenTxs.Add(fc.Txs)
	}
	evictedChunks := vm.seenChunks.SetMin(blkTime)
	vm.Logger().Debug("chunks evicted from seen", zap.Int("len", len(evictedChunks)))
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

	// Update timestamp in mempool
	//
	// We rely on the [vm.waiters] map to notify listeners of dropped
	// transactions instead of the mempool because we won't need to iterate
	// through as many transactions.
	vm.mempool.SetMinTimestamp(ctx, blkTime)

	// Enqueue block for processing
	vm.acceptedQueue <- &acceptedWrapper{b, chunks}
}

func (vm *VM) CacheValidators(ctx context.Context, height uint64) {
	vm.proposerMonitor.Fetch(ctx, height)
}

func (vm *VM) AddressPartition(ctx context.Context, height uint64, addr codec.Address) (ids.NodeID, error) {
	return vm.proposerMonitor.AddressPartition(ctx, height, addr)
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

func (vm *VM) AcceptedSyncableBlock(
	ctx context.Context,
	sb *chain.SyncableBlock,
) (block.StateSyncMode, error) {
	return vm.stateSyncClient.AcceptedSyncableBlock(ctx, sb)
}

func (vm *VM) StateReady() bool {
	if vm.stateSyncClient == nil {
		// Can occur in test
		return false
	}
	return vm.stateSyncClient.StateReady()
}

func (vm *VM) UpdateSyncTarget(b *chain.StatelessBlock) (bool, error) {
	return vm.stateSyncClient.UpdateSyncTarget(b)
}

func (vm *VM) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return vm.stateSyncClient.GetOngoingSyncStateSummary(ctx)
}

func (vm *VM) StateSyncEnabled(ctx context.Context) (bool, error) {
	return vm.stateSyncClient.StateSyncEnabled(ctx)
}

func (vm *VM) StateManager() chain.StateManager {
	return vm.c.StateManager()
}

func (vm *VM) RecordWaitAuth(t time.Duration) {
	vm.metrics.waitAuth.Observe(float64(t))
}

func (vm *VM) RecordWaitCommit(t time.Duration) {
	vm.metrics.waitCommit.Observe(float64(t))
}

func (vm *VM) RecordWaitExec(t time.Duration) {
	vm.metrics.waitExec.Observe(float64(t))
}

func (vm *VM) RecordStateChanges(c int) {
	vm.metrics.stateChanges.Add(float64(c))
}

func (vm *VM) RecordStateOperations(c int) {
	vm.metrics.stateOperations.Add(float64(c))
}

func (vm *VM) GetVerifyAuth() bool {
	return vm.config.GetVerifyAuth()
}

func (vm *VM) GetAuthVerifyCores() int {
	return vm.config.GetAuthVerificationCores()
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

func (vm *VM) RecordTxsValid(c int) {
	vm.metrics.txsValid.Add(float64(c))
}

func (vm *VM) GetTargetBuildDuration() time.Duration {
	return vm.config.GetTargetBuildDuration()
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

func (vm *VM) RecordClearedMempool() {
	vm.metrics.clearedMempool.Inc()
}

func (vm *VM) UnitPrices(context.Context) (chain.Dimensions, error) {
	return vm.Rules(time.Now().UnixMilli()).GetUnitPrices(), nil
}

func (vm *VM) GetTransactionExecutionCores() int {
	return vm.config.GetTransactionExecutionCores()
}

func (vm *VM) GetExecutorRecorder() executor.Metrics {
	return vm.metrics.executorRecorder
}

func (vm *VM) NextChunkCertificate(ctx context.Context) (*chain.ChunkCertificate, bool) {
	return vm.cm.NextChunkCertificate(ctx)
}

func (vm *VM) RestoreChunkCertificates(ctx context.Context, certs []*chain.ChunkCertificate) {
	vm.cm.RestoreChunkCertificates(ctx, certs)
}

func (vm *VM) Engine() *chain.Engine {
	return vm.engine
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

func (vm *VM) RequestChunks(certs []*chain.ChunkCertificate, chunks chan *chain.Chunk) {
	vm.cm.RequestChunks(certs, chunks)
}
