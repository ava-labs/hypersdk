// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/builder"
	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/gossiper"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ chain.VM              = (*VM)(nil)
	_ gossiper.VM           = (*VM)(nil)
	_ builder.VM            = (*VM)(nil)
	_ block.ChainVM         = (*VM)(nil)
	_ block.StateSyncableVM = (*VM)(nil)
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

func (vm *VM) ActionCodec() *codec.TypeParser[chain.Action] {
	return vm.actionCodec
}

func (vm *VM) OutputCodec() *codec.TypeParser[codec.Typed] {
	return vm.outputCodec
}

func (vm *VM) AuthCodec() *codec.TypeParser[chain.Auth] {
	return vm.authCodec
}

func (vm *VM) AuthVerifiers() workers.Workers {
	return vm.authVerifiers
}

func (vm *VM) Tracer() trace.Tracer {
	return vm.tracer
}

func (vm *VM) Logger() logging.Logger {
	return vm.snowCtx.Log
}

func (vm *VM) Rules(t int64) chain.Rules {
	return vm.ruleFactory.GetRules(t)
}

func (vm *VM) LastAcceptedBlock() *chain.StatefulBlock {
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

func (vm *VM) ImmutableState(ctx context.Context) (state.Immutable, error) {
	ts := tstate.New(0)
	state, err := vm.State()
	if err != nil {
		return nil, err
	}
	return ts.ExportMerkleDBView(ctx, vm.tracer, state)
}

func (vm *VM) Mempool() chain.Mempool {
	return vm.mempool
}

func (vm *VM) IsRepeat(ctx context.Context, txs []*chain.Transaction, marker set.Bits, stop bool) set.Bits {
	_, span := vm.tracer.Start(ctx, "VM.IsRepeat")
	defer span.End()

	return vm.seen.Contains(txs, marker, stop)
}

func (vm *VM) Verified(ctx context.Context, b *chain.StatefulBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Verified")
	defer span.End()

	vm.metrics.txsVerified.Add(float64(len(b.Txs)))
	vm.verifiedL.Lock()
	vm.verifiedBlocks[b.ID()] = b
	vm.verifiedL.Unlock()
	vm.parsedBlocks.Evict(b.ID())
	vm.mempool.Remove(ctx, b.Txs)
	vm.gossiper.BlockVerified(b.Tmstmp)
	vm.checkActivity(ctx)

	if b.Processed() {
		fm := b.FeeManager()
		vm.snowCtx.Log.Info(
			"verified block",
			zap.Stringer("blkID", b.ID()),
			zap.Uint64("height", b.Hght),
			zap.Int("txs", len(b.Txs)),
			zap.Stringer("parent root", b.StateRoot),
			zap.Bool("state ready", vm.StateReady()),
			zap.Any("unit prices", fm.UnitPrices()),
			zap.Any("units consumed", fm.UnitsConsumed()),
		)
	} else {
		// [b.FeeManager] is not populated if the block
		// has not been processed.
		vm.snowCtx.Log.Info(
			"skipped block verification",
			zap.Stringer("blkID", b.ID()),
			zap.Uint64("height", b.Hght),
			zap.Int("txs", len(b.Txs)),
			zap.Stringer("parent root", b.StateRoot),
			zap.Bool("state ready", vm.StateReady()),
		)
	}
}

func (vm *VM) Rejected(ctx context.Context, b *chain.StatefulBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Rejected")
	defer span.End()

	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()
	vm.mempool.Add(ctx, b.Txs)

	// Ensure children of block are cleared, they may never be
	// verified
	vm.snowCtx.Log.Info("rejected block", zap.Stringer("id", b.ID()))
}

func (vm *VM) processAcceptedBlock(b *chain.StatefulBlock) {
	start := time.Now()
	defer func() {
		vm.metrics.blockProcess.Observe(float64(time.Since(start)))
	}()

	// We skip blocks that were not processed because metadata required to
	// process blocks opaquely (like looking at results) is not populated.
	//
	// We don't need to worry about dangling messages in listeners because we
	// don't allow subscription until the node is healthy.
	if !b.Processed() {
		vm.snowCtx.Log.Info("skipping unprocessed block", zap.Uint64("height", b.Hght))
		return
	}

	// TODO: consider removing this (unused and requires an extra iteration)
	for _, tx := range b.Txs {
		// Only cache auth for accepted blocks to prevent cache manipulation from RPC submissions
		vm.cacheAuth(tx.Auth)
	}

	// Update price metrics
	feeManager := b.FeeManager()
	vm.metrics.bandwidthPrice.Set(float64(feeManager.UnitPrice(fees.Bandwidth)))
	vm.metrics.computePrice.Set(float64(feeManager.UnitPrice(fees.Compute)))
	vm.metrics.storageReadPrice.Set(float64(feeManager.UnitPrice(fees.StorageRead)))
	vm.metrics.storageAllocatePrice.Set(float64(feeManager.UnitPrice(fees.StorageAllocate)))
	vm.metrics.storageWritePrice.Set(float64(feeManager.UnitPrice(fees.StorageWrite)))

	// Subscriptions must be updated before setting the last processed height
	// key to guarantee at-least-once delivery semantics
	executedBlock := chain.NewExecutedBlockFromStateful(b)
	for _, subscription := range vm.blockSubscriptions {
		if err := subscription.Accept(executedBlock); err != nil {
			vm.Fatal("subscription failed to process block", zap.Error(err))
		}
	}

	if err := vm.SetLastProcessedHeight(b.Height()); err != nil {
		vm.Fatal("failed to update the last processed height", zap.Error(err))
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
	for b := range vm.acceptedQueue {
		vm.processAcceptedBlock(b)
		vm.snowCtx.Log.Info(
			"block processed",
			zap.Stringer("blkID", b.ID()),
			zap.Uint64("height", b.Hght),
		)
	}
}

func (vm *VM) Accepted(ctx context.Context, b *chain.StatefulBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Accepted")
	defer span.End()

	vm.metrics.txsAccepted.Add(float64(len(b.Txs)))

	// Update accepted blocks on-disk and caches
	if err := vm.UpdateLastAccepted(b); err != nil {
		vm.Fatal("unable to update last accepted", zap.Error(err))
	}

	// Remove from verified caches
	//
	// We do this after setting [lastAccepted] to avoid
	// a race where the block isn't accessible.
	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()

	// Update replay protection heap
	//
	// Transactions are added to [seen] with their [expiry], so we don't need to
	// transform [blkTime] when calling [SetMin] here.
	blkTime := b.Tmstmp
	evicted := vm.seen.SetMin(blkTime)
	vm.Logger().Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	vm.seen.Add(b.Txs)

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
	removed := vm.mempool.SetMinTimestamp(ctx, blkTime)

	// Enqueue block for processing
	vm.acceptedQueue <- b

	vm.snowCtx.Log.Info(
		"accepted block",
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Hght),
		zap.Int("txs", len(b.Txs)),
		zap.Stringer("parent root", b.StateRoot),
		zap.Int("size", len(b.Bytes())),
		zap.Int("dropped mempool txs", len(removed)),
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

func (vm *VM) NodeID() ids.NodeID {
	return vm.snowCtx.NodeID
}

func (vm *VM) PreferredBlock(ctx context.Context) (*chain.StatefulBlock, error) {
	return vm.GetStatefulBlock(ctx, vm.preferred)
}

func (vm *VM) PreferredHeight(ctx context.Context) (uint64, error) {
	preferredBlk, err := vm.GetStatefulBlock(ctx, vm.preferred)
	if err != nil {
		return 0, err
	}
	return preferredBlk.Hght, nil
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

func (vm *VM) UpdateSyncTarget(b *chain.StatefulBlock) (bool, error) {
	return vm.stateSyncClient.UpdateSyncTarget(b)
}

func (vm *VM) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return vm.stateSyncClient.GetOngoingSyncStateSummary(ctx)
}

func (vm *VM) StateSyncEnabled(ctx context.Context) (bool, error) {
	return vm.stateSyncClient.StateSyncEnabled(ctx)
}

func (vm *VM) Genesis() genesis.Genesis {
	return vm.genesis
}

func (vm *VM) BalanceHandler() chain.BalanceHandler {
	return vm.balanceHandler
}

func (vm *VM) MetadataManager() chain.MetadataManager {
	return vm.metadataManager
}

func (vm *VM) RecordRootCalculated(t time.Duration) {
	vm.metrics.rootCalculated.Observe(float64(t))
}

func (vm *VM) RecordWaitRoot(t time.Duration) {
	vm.metrics.waitRoot.Observe(float64(t))
}

func (vm *VM) RecordWaitSignatures(t time.Duration) {
	vm.metrics.waitSignatures.Observe(float64(t))
}

func (vm *VM) RecordStateChanges(c int) {
	vm.metrics.stateChanges.Add(float64(c))
}

func (vm *VM) RecordStateOperations(c int) {
	vm.metrics.stateOperations.Add(float64(c))
}

func (vm *VM) GetVerifyAuth() bool {
	return vm.config.VerifyAuth
}

func (vm *VM) RecordTxsGossiped(c int) {
	vm.metrics.txsGossiped.Add(float64(c))
}

func (vm *VM) RecordTxsReceived(c int) {
	vm.metrics.txsReceived.Add(float64(c))
}

func (vm *VM) RecordSeenTxsReceived(c int) {
	vm.metrics.seenTxsReceived.Add(float64(c))
}

func (vm *VM) RecordBuildCapped() {
	vm.metrics.buildCapped.Inc()
}

func (vm *VM) GetTargetBuildDuration() time.Duration {
	return vm.config.TargetBuildDuration
}

func (vm *VM) GetTargetGossipDuration() time.Duration {
	return vm.config.TargetGossipDuration
}

func (vm *VM) RecordEmptyBlockBuilt() {
	vm.metrics.emptyBlockBuilt.Inc()
}

func (vm *VM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	bv, ok := vm.authEngine[authTypeID]
	if !ok {
		return nil, false
	}
	return bv.GetBatchVerifier(cores, count), ok
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

func (vm *VM) RecordClearedMempool() {
	vm.metrics.clearedMempool.Inc()
}

func (vm *VM) UnitPrices(context.Context) (fees.Dimensions, error) {
	v, err := vm.stateDB.Get(chain.FeeKey(vm.MetadataManager().FeePrefix()))
	if err != nil {
		return fees.Dimensions{}, err
	}
	return internalfees.NewManager(v).UnitPrices(), nil
}

func (vm *VM) GetTransactionExecutionCores() int {
	return vm.config.TransactionExecutionCores
}

func (vm *VM) GetStateFetchConcurrency() int {
	return vm.config.StateFetchConcurrency
}

func (vm *VM) GetExecutorBuildRecorder() executor.Metrics {
	return vm.metrics.executorBuildRecorder
}

func (vm *VM) GetExecutorVerifyRecorder() executor.Metrics {
	return vm.metrics.executorVerifyRecorder
}
