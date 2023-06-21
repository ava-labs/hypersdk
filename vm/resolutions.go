// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/workers"
)

var (
	_ chain.VM                           = (*VM)(nil)
	_ gossiper.VM                        = (*VM)(nil)
	_ builder.VM                         = (*VM)(nil)
	_ block.ChainVM                      = (*VM)(nil)
	_ block.HeightIndexedChainVM         = (*VM)(nil)
	_ block.StateSyncableVM              = (*VM)(nil)
	_ block.BuildBlockWithContextChainVM = (*VM)(nil)
)

func (vm *VM) HRP() string {
	return vm.genesis.GetHRP()
}

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

func (vm *VM) Workers() *workers.Workers {
	return vm.workers
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

func (vm *VM) LastAcceptedBlock() *chain.StatelessRootBlock {
	return vm.lastAccepted
}

func (vm *VM) LastProcessedBlock() *chain.StatelessRootBlock {
	return vm.lastProcessed
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

func (vm *VM) Mempool() chain.Mempool {
	return vm.mempool
}

func (vm *VM) CollectRepeats(ctx context.Context, txs []*chain.Transaction, repeats *set.Bits) {
	_, span := vm.tracer.Start(ctx, "VM.CollectRepeats")
	defer span.End()

	vm.seen.CollectRepeats(txs, repeats)
}

func (vm *VM) Verified(ctx context.Context, b *chain.StatelessRootBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Verified")
	defer span.End()

	blkTxs := 0
	for _, txBlk := range b.GetTxBlocks() {
		blkTxs += len(txBlk.Txs)
	}
	vm.metrics.txsVerified.Add(float64(blkTxs))
	vm.metrics.txBlocksVerified.Add(float64(len(b.TxBlocks)))
	vm.verifiedL.Lock()
	vm.verifiedBlocks[b.ID()] = b
	vm.verifiedL.Unlock()
	vm.parsedBlocks.Evict(b.ID())
	vm.gossiper.BlockVerified(b.Tmstmp)
	vm.snowCtx.Log.Info(
		"verified block",
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Hght),
		zap.Int("txs", blkTxs),
		zap.Bool("state ready", vm.StateReady()),
	)
}

func (vm *VM) Rejected(ctx context.Context, b *chain.StatelessRootBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Rejected")
	defer span.End()

	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()
	for _, txBlock := range b.GetTxBlocks() {
		vm.mempool.Add(ctx, txBlock.Txs)
	}

	// TODO: handle async?
	if err := vm.c.Rejected(ctx, b); err != nil {
		vm.snowCtx.Log.Fatal("rejected processing failed", zap.Error(err))
	}

	// Ensure children of block are cleared, they may never be
	// verified
	vm.snowCtx.Log.Info("rejected block", zap.Stringer("id", b.ID()))
}

func (vm *VM) processAcceptedBlocks() {
	// The VM closes [acceptedQueue] during shutdown. We wait for all enqueued blocks
	// to be processed before returning as a guarantee to listeners (which may
	// persist indexed state) instead of just exiting as soon as `vm.stop` is
	// closed.
	for b := range vm.acceptedQueue {
		// Process block (can just have chunks be txs + height)
		start := time.Now()
		if err := b.Execute(context.TODO(), vm.lastProcessed); err != nil {
			vm.Logger().Fatal("unable to execute block", zap.Error(err))
			return
		}
		vm.metrics.rootBlockExecute.Observe(float64(time.Since(start)))

		// Update replay protection heap
		//
		// We check against seen during "Execute" so we need to do this after we've verified txs in the block.
		blkTime := b.Tmstmp
		vm.seen.SetMin(blkTime)
		for _, txBlock := range b.GetTxBlocks() {
			vm.seen.Add(txBlock.Txs)
		}

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

		vm.metrics.unitsAccepted.Add(float64(b.UnitsConsumed()))
		vm.lastProcessed = b

		// Update TxBlock store
		batch := vm.vmDB.NewBatch()
		for _, txBlock := range b.GetTxBlocks() {
			if err := vm.StoreTxBlock(batch, txBlock); err != nil {
				vm.snowCtx.Log.Fatal("unable to store tx block", zap.Error(err))
			}
			txBlock.Free() // clears memory from execute
		}
		if b.Hght > vm.config.GetRootBlockPruneDiff() { // > ensures we don't prune genesis
			prunableHeight := b.Hght - vm.config.GetRootBlockPruneDiff()
			bid, err := vm.GetDiskBlockIDAtHeight(prunableHeight)
			if err == nil {
				rootBlock, err := vm.GetDiskBlock(bid)
				if err == nil {
					for i := 0; i < len(rootBlock.TxBlocks); i++ {
						if err := vm.DeleteTxBlock(batch, rootBlock.MinTxHght+uint64(i)); err != nil {
							vm.snowCtx.Log.Fatal("unable to delete tx block", zap.Error(err))
							return
						}
						vm.metrics.deletedTxBlocks.Inc()
					}
				} else {
					vm.snowCtx.Log.Debug("not deleting tx blocks at height", zap.Error(err))
				}
			} else {
				vm.snowCtx.Log.Debug("not deleting tx blocks at height", zap.Error(err))
			}
		}
		if err := batch.Write(); err != nil {
			vm.snowCtx.Log.Fatal("unable to commit tx block batch", zap.Error(err))
			return
		}
		vm.txBlockManager.Accept(b.MaxTxHght())

		// TODO: remove old txBlocks from disk (delta off of root block)

		// We skip blocks that were not processed because metadata required to
		// process blocks opaquely (like looking at results) is not populated.
		//
		// We don't need to worry about dangling messages in listeners because we
		// don't allow subscription until the node is healthy.
		if !b.Processed() {
			vm.snowCtx.Log.Info("skipping unprocessed block", zap.Uint64("height", b.Hght))
			continue
		}

		// Update controller
		if err := vm.c.Accepted(context.TODO(), b); err != nil {
			vm.snowCtx.Log.Fatal("accepted processing failed", zap.Error(err))
			return
		}

		// Sign and store any warp messages (regardless if validator now, may become one)
		if b.ContainsWarp {
			results := b.Results()
			count := 0
			for _, txBlock := range b.GetTxBlocks() {
				for _, tx := range txBlock.Txs {
					result := results[count]
					count++

					if result.WarpMessage == nil {
						// failed execution
						continue
					}
					start := time.Now()
					signature, err := vm.snowCtx.WarpSigner.Sign(result.WarpMessage)
					if err != nil {
						vm.snowCtx.Log.Fatal("unable to sign warp message", zap.Error(err))
						return
					}
					if err := vm.StoreWarpSignature(tx.ID(), vm.snowCtx.PublicKey, signature); err != nil {
						vm.snowCtx.Log.Fatal("unable to store warp signature", zap.Error(err))
						return
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
			}
		}

		// Update server
		if err := vm.webSocketServer.AcceptBlock(b); err != nil {
			vm.snowCtx.Log.Fatal("unable to accept block in websocket server", zap.Error(err))
			return
		}
		// Must clear accepted txs before [SetMinTx] or else we will errnoueously
		// send [ErrExpired] messages.
		if err := vm.webSocketServer.SetMinTx(b.Tmstmp); err != nil {
			vm.snowCtx.Log.Fatal("unable to set min tx in websocket server", zap.Error(err))
			return
		}
		vm.snowCtx.Log.Info(
			"accepted block processed",
			zap.Stringer("blkID", b.ID()),
			zap.Uint64("height", b.Hght),
		)
		vm.metrics.acceptorDrift.Set(float64(vm.lastAccepted.Hght - b.Hght))
	}
	close(vm.acceptorDone)
	vm.snowCtx.Log.Info("acceptor queue shutdown")
}

func (vm *VM) Accepted(ctx context.Context, b *chain.StatelessRootBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Accepted")
	defer span.End()

	blkTxs := 0
	for _, txBlk := range b.GetTxBlocks() {
		blkTxs += len(txBlk.Txs)
	}
	vm.metrics.txsAccepted.Add(float64(blkTxs))
	vm.metrics.txBlocksAccepted.Add(float64(len(b.TxBlocks)))
	vm.blocks.Put(b.ID(), b)
	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()
	vm.lastAccepted = b

	// Enqueue block for processing
	vm.acceptedQueue <- b

	vm.snowCtx.Log.Info(
		"accepted block",
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Hght),
		zap.Int("txs", blkTxs),
		zap.Int("size", len(b.Bytes())),
		zap.Bool("state ready", vm.StateReady()),
	)
}

func (vm *VM) IsValidator(ctx context.Context, nid ids.NodeID) (bool, error) {
	return vm.proposerMonitor.IsValidator(ctx, nid)
}

func (vm *VM) Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error) {
	return vm.proposerMonitor.Proposers(ctx, diff, depth)
}

func (vm *VM) CurrentValidators(
	ctx context.Context,
) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{}) {
	return vm.proposerMonitor.Validators(ctx)
}

func (vm *VM) GatherSignatures(ctx context.Context, txID ids.ID, msg []byte) {
	vm.warpManager.GatherSignatures(ctx, txID, msg)
}

func (vm *VM) NodeID() ids.NodeID {
	return vm.snowCtx.NodeID
}

func (vm *VM) PreferredBlock(ctx context.Context) (*chain.StatelessRootBlock, error) {
	return vm.GetStatelessRootBlock(ctx, vm.preferred)
}

func (vm *VM) StopChan() chan struct{} {
	return vm.stop
}

func (vm *VM) EngineChan() chan<- common.Message {
	return vm.toEngine
}

// Used for integration and load testing
func (vm *VM) Builder() builder.Builder {
	return vm.builder
}

func (vm *VM) Gossiper() gossiper.Gossiper {
	return vm.gossiper
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

func (vm *VM) UpdateSyncTarget(b *chain.StatelessRootBlock) (bool, error) {
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

func (vm *VM) RecordRootCalculated(t time.Duration) {
	vm.metrics.rootCalculated.Observe(float64(t))
}

func (vm *VM) RecordCommitState(t time.Duration) {
	vm.metrics.commitState.Observe(float64(t))
}

func (vm *VM) RecordWaitSignatures(t time.Duration) {
	vm.metrics.waitSignatures.Observe(float64(t))
}

func (vm *VM) RecordVerifyWait(t time.Duration) {
	vm.metrics.verifyWait.Observe(float64(t))
}

func (vm *VM) RecordTxBlockVerify(t time.Duration) {
	vm.metrics.txBlockVerify.Observe(float64(t))
}

func (vm *VM) RecordTxBlockIssuanceDiff(t time.Duration) {
	vm.metrics.txBlockIssuanceDiff.Observe(float64(t))
}

func (vm *VM) RecordRootBlockIssuanceDiff(t time.Duration) {
	vm.metrics.rootBlockIssuanceDiff.Observe(float64(t))
}

func (vm *VM) RecordRootBlockAcceptanceDiff(t time.Duration) {
	vm.metrics.rootBlockAcceptanceDiff.Observe(float64(t))
}

func (vm *VM) RecordStateChanges(c int) {
	vm.metrics.stateChanges.Add(float64(c))
}

func (vm *VM) RecordTxBlocksMissing(c int) {
	vm.metrics.txBlocksMissing.Add(float64(c))
}

func (vm *VM) RecordStateOperations(c int) {
	vm.metrics.stateOperations.Add(float64(c))
}

func (vm *VM) RecordEarlyBuildStop() {
	vm.metrics.earlyBuildStop.Inc()
}

func (vm *VM) RecordTxFailedExecution() {
	vm.metrics.txFailedExecution.Inc()
}

func (vm *VM) IssueTxBlock(ctx context.Context, blk *chain.StatelessTxBlock) {
	vm.txBlockManager.IssueTxBlock(ctx, blk)
}

func (vm *VM) RequireTxBlocks(ctx context.Context, minHght uint64, blks []ids.ID) int {
	return vm.txBlockManager.RequireTxBlocks(ctx, minHght, blks)
}

func (vm *VM) RetryVerify(ctx context.Context, blks []ids.ID) {
	vm.txBlockManager.RetryVerify(ctx, blks)
}

func (vm *VM) GetStatelessTxBlock(ctx context.Context, blkID ids.ID, hght uint64) (*chain.StatelessTxBlock, error) {
	blk := vm.txBlockManager.txBlocks.Get(blkID)
	if blk != nil {
		if !blk.verified {
			return nil, fmt.Errorf("blk not verified; height=%d id=%s", blk.blk.Hght, blkID)
		}
		return blk.blk, nil
	}
	return vm.GetTxBlock(hght)
}

func (vm *VM) GetVerifySignatures() bool {
	return vm.config.GetVerifySignatures()
}

func (vm *VM) GetMinBuildTime() time.Duration {
	return vm.config.GetMinBuildTime()
}

func (vm *VM) GetMaxBuildTime() time.Duration {
	return vm.config.GetMaxBuildTime()
}

func (vm *VM) RecordMempoolSizeAfterBuild(size int) {
	vm.metrics.mempoolSizeAfterBuild.Set(float64(size))
}

func (vm *VM) RecordTxsAttempted(attempted int) {
	vm.metrics.txsAttempted.Add(float64(attempted))
}

func (vm *VM) IsBuilding() bool {
	return vm.building
}

func (vm *VM) RecordGossipTrigger() {
	vm.metrics.txGossipTriggered.Inc()
}

func (vm *VM) RecordBuildSelect(t time.Duration) {
	vm.metrics.buildSelect.Observe(float64(t))
}

func (vm *VM) RecordBuildMarshal(t time.Duration) {
	vm.metrics.buildMarshal.Observe(float64(t))
}
