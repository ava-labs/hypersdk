// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/builder"
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/gossiper"
	"github.com/AnomalyFi/hypersdk/workers"
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

func (vm *VM) LastAcceptedBlock() *chain.StatelessBlock {
	return vm.lastAccepted
}

func (vm *VM) IsBootstrapped() bool {
	return vm.bootstrapped.Get()
}

func (vm *VM) State() (*merkledb.Database, error) {
	// As soon as synced (before ready), we can safely request data from the db.
	if !vm.StateReady() {
		return nil, ErrStateMissing
	}
	return vm.stateDB, nil
}

func (vm *VM) Mempool() chain.Mempool {
	return vm.mempool
}

func (vm *VM) IsRepeat(ctx context.Context, txs []*chain.Transaction) bool {
	_, span := vm.tracer.Start(ctx, "VM.IsRepeat")
	defer span.End()

	return vm.seen.Any(txs)
}

func (vm *VM) Verified(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Verified")
	defer span.End()

	vm.metrics.unitsVerified.Add(float64(b.UnitsConsumed))
	vm.metrics.txsVerified.Add(float64(len(b.Txs)))
	vm.verifiedL.Lock()
	vm.verifiedBlocks[b.ID()] = b
	vm.verifiedL.Unlock()
	vm.parsedBlocks.Evict(b.ID())
	vm.mempool.Remove(ctx, b.Txs)
	vm.gossiper.BlockVerified(b.Tmstmp)

	for _, tx := range b.Txs {
		vm.snowCtx.Log.Info("Verified tx action data is:", zap.Stringer("type_of_action", reflect.TypeOf(tx.Action)))
	}

	vm.snowCtx.Log.Info(
		"verified block",
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Hght),
		zap.Int("txs", len(b.Txs)),
		zap.Bool("state ready", vm.StateReady()),
	)
}

func (vm *VM) Rejected(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Rejected")
	defer span.End()

	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()
	vm.mempool.Add(ctx, b.Txs)

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
		}

		// Sign and store any warp messages (regardless if validator now, may become one)
		results := b.Results()
		for i, tx := range b.Txs {
			result := results[i]
			if result.WarpMessage == nil {
				continue
			}
			start := time.Now()
			signature, err := vm.snowCtx.WarpSigner.Sign(result.WarpMessage)
			if err != nil {
				vm.snowCtx.Log.Fatal("unable to sign warp message", zap.Error(err))
			}
			if err := vm.StoreWarpSignature(tx.ID(), vm.snowCtx.PublicKey, signature); err != nil {
				vm.snowCtx.Log.Fatal("unable to store warp signature", zap.Error(err))
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

		// Update server
		if err := vm.webSocketServer.AcceptBlock(b); err != nil {
			vm.snowCtx.Log.Fatal("unable to accept block in websocket server", zap.Error(err))
		}
		// Must clear accepted txs before [SetMinTx] or else we will errnoueously
		// send [ErrExpired] messages.
		if err := vm.webSocketServer.SetMinTx(b.Tmstmp); err != nil {
			vm.snowCtx.Log.Fatal("unable to set min tx in websocket server", zap.Error(err))
		}
		vm.snowCtx.Log.Info(
			"block processed",
			zap.Stringer("blkID", b.ID()),
			zap.Uint64("height", b.Hght),
		)
	}
	close(vm.acceptorDone)
	vm.snowCtx.Log.Info("acceptor queue shutdown")
}

func (vm *VM) Accepted(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Accepted")
	defer span.End()

	vm.metrics.unitsAccepted.Add(float64(b.UnitsConsumed))
	vm.metrics.txsAccepted.Add(float64(len(b.Txs)))
	vm.blocks.Put(b.ID(), b)
	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()
	vm.lastAccepted = b

	// Update replay protection heap
	blkTime := b.Tmstmp
	vm.seen.SetMin(blkTime)
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

	for _, tx := range b.Txs {
		vm.snowCtx.Log.Info("tx action data is:", zap.Stringer("type_of_action", reflect.TypeOf(tx.Action)))
	}
	//TODO this is the last step so I want to print out the transactions actions at this point
	vm.snowCtx.Log.Info(
		"accepted block",
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Hght),
		zap.Int("txs", len(b.Txs)),
		zap.Int("size", len(b.Bytes())),
		zap.Uint64("units", b.UnitsConsumed),
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

func (vm *VM) RecordRootCalculated(t time.Duration) {
	vm.metrics.rootCalculated.Observe(float64(t))
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
