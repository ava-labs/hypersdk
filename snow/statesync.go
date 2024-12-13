// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/x/merkledb"
	hcontext "github.com/ava-labs/hypersdk/context"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/prometheus/client_golang/prometheus"
)

var _ block.StateSyncableVM = (*VM[Block, Block, Block])(nil)

func (o *Options[I, O, A]) WithStateSyncableVM(
	client *statesync.Client[*StatefulBlock[I, O, A]],
	server *statesync.Server[*StatefulBlock[I, O, A]],
) {
	o.StateSyncClient = client
	o.StateSyncServer = server
}

type StateSyncConfig struct {
	MinBlocks   uint64 `json:"minBlocks"`
	Parallelism int    `json:"parallelism"`
}

func NewDefaultStateSyncConfig() StateSyncConfig {
	return StateSyncConfig{
		MinBlocks:   768,
		Parallelism: 4,
	}
}

func GetStateSyncConfig(ctx *hcontext.Context) (StateSyncConfig, error) {
	return hcontext.GetConfigFromContext(ctx, "statesync", NewDefaultStateSyncConfig())
}

func (o *Options[I, O, A]) WithStateSyncer(
	db database.Database,
	stateDB merkledb.MerkleDB,
	rangeProofHandlerID uint64,
	changeProofHandlerID uint64,
	branchFactor merkledb.BranchFactor,
) error {
	server := statesync.NewServer[*StatefulBlock[I, O, A]](
		o.vm.log,
		o.vm.covariantVM,
	)
	o.StateSyncServer = server

	syncerRegistry := prometheus.NewRegistry()
	if err := o.vm.snowCtx.Metrics.Register("syncer", syncerRegistry); err != nil {
		return err
	}
	stateSyncConfig, err := GetStateSyncConfig(o.vm.hctx)
	if err != nil {
		return err
	}
	client := statesync.NewClient[*StatefulBlock[I, O, A]](
		o.vm.covariantVM,
		o.vm.snowCtx.Log,
		syncerRegistry,
		db,
		stateDB,
		o.Network,
		rangeProofHandlerID,
		changeProofHandlerID,
		branchFactor,
		stateSyncConfig.MinBlocks,
		stateSyncConfig.Parallelism,
	)
	o.StateSyncClient = client
	o.OnNormalOperationStarted = append(o.OnNormalOperationStarted, client.StartBootstrapping)
	// Note: this is not perfect because we may need to get a notification of a block between finishing state sync
	// and when the engine/VM has received the notification and switched over.
	// o.WithPreReadyAcceptedSub(event.SubscriptionFunc[I]{
	// 	NotifyF: func(ctx context.Context, block I) error {
	// 		_, err := client.UpdateSyncTarget(block)
	// 		return err
	// 	},
	// })
	return statesync.RegisterHandlers(o.vm.log, o.Network, rangeProofHandlerID, changeProofHandlerID, stateDB)
}

// FinishStateSync is responsible for setting the last accepted block of the VM after state sync completes.
// This function must grab the lock because it's called from a thread the VM controls instead of the consensus
// engine.
func (v *VM[I, O, A]) FinishStateSync(ctx context.Context, input I, output O, accepted A) error {
	v.snowCtx.Lock.Lock()
	defer v.snowCtx.Lock.Unlock()

	// Caller must guarantee that the VM has been marked as ready before calling FinishStateSync
	// TODO: switch from Ready to using FinishStateSync to mark the VM as ready.
	if !v.Options.Ready.Ready() {
		return fmt.Errorf("can't finish state sync on block %s because the VM is not ready", input)
	}

	// If the block is already the last accepted block, update the fields and return
	if input.ID() == v.lastAcceptedBlock.ID() {
		v.lastAcceptedBlock.setAccepted(output, accepted)
		return nil
	}

	// Dynamic state sync notifies completion async, so we may have verified/accepted new blocks in
	// the interim (before successfully grabbing the lock).
	// Create the block and reprocess all blocks in the range (blk, lastAcceptedBlock]
	blk := NewAcceptedBlock(v.covariantVM, input, output, accepted)
	reprocessBlks, err := v.covariantVM.getExclusiveBlockRange(ctx, blk, v.lastAcceptedBlock)
	if err != nil {
		return fmt.Errorf("failed to get block range while completing state sync: %w", err)
	}
	// Guarantee that parent is fully populated, so we can correctly verify/accept each
	// block up to and including the last accepted block.
	reprocessBlks = append(reprocessBlks, v.lastAcceptedBlock)
	parent := blk
	for _, reprocessBlk := range reprocessBlks {
		if err := reprocessBlk.verify(ctx, parent.Output); err != nil {
			return fmt.Errorf("failed to finish state sync while verifying block %s in range (%s, %s): %w", reprocessBlk, blk, v.lastAcceptedBlock, err)
		}
		if err := reprocessBlk.accept(ctx); err != nil {
			return fmt.Errorf("failed to finish state sync while accepting block %s in range (%s, %s): %w", reprocessBlk, blk, v.lastAcceptedBlock, err)
		}
	}

	return nil
}

func (v *VM[I, O, A]) StateSyncEnabled(ctx context.Context) (bool, error) {
	return v.Options.StateSyncClient.StateSyncEnabled(ctx)
}

func (v *VM[I, O, A]) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.Options.StateSyncClient.GetOngoingSyncStateSummary(ctx)
}

func (v *VM[I, O, A]) GetLastStateSummary(ctx context.Context) (block.StateSummary, error) {
	return v.Options.StateSyncServer.GetLastStateSummary(ctx)
}

func (v *VM[I, O, A]) ParseStateSummary(ctx context.Context, summaryBytes []byte) (block.StateSummary, error) {
	return v.Options.StateSyncClient.ParseStateSummary(ctx, summaryBytes)
}

func (v *VM[I, O, A]) GetStateSummary(ctx context.Context, summaryHeight uint64) (block.StateSummary, error) {
	return v.Options.StateSyncServer.GetStateSummary(ctx, summaryHeight)
}
