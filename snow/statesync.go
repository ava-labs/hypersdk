// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/x/merkledb"
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
	stateSyncConfig, err := GetConfig(o.vm.config, "statesync", NewDefaultStateSyncConfig())
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
	o.WithReady(o.StateSyncClient)
	return statesync.RegisterHandlers(o.vm.log, o.Network, rangeProofHandlerID, changeProofHandlerID, stateDB)
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
