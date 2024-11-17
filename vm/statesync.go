// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	rangeProofHandlerID  = 0
	changeProofHandlerID = 1
)

var _ block.StateSyncableVM = (*VM)(nil)

func (vm *VM) initStateSync() error {
	// Setup state syncing
	vm.StateSyncServer = statesync.NewServer[*StatefulBlock](
		vm.snowCtx.Log,
		vm,
	)
	syncerRegistry := prometheus.NewRegistry()
	if err := vm.snowCtx.Metrics.Register("syncer", syncerRegistry); err != nil {
		vm.snowCtx.Log.Error("could not register syncer metrics", zap.Error(err))
		return err
	}
	vm.StateSyncClient = statesync.NewClient[*StatefulBlock](
		vm,
		vm.snowCtx.Log,
		syncerRegistry,
		vm.stateDB,
		vm.network,
		rangeProofHandlerID,
		changeProofHandlerID,
		vm.genesis.GetStateBranchFactor(),
		vm.config.StateSyncMinBlocks,
		vm.config.StateSyncParallelism,
	)
	if err := statesync.RegisterHandlers(vm.snowCtx.Log, vm.network, rangeProofHandlerID, changeProofHandlerID, vm.stateDB); err != nil {
		return err
	}

	if err := vm.network.AddHandler(
		txGossipHandlerID,
		NewTxGossipHandler(vm),
	); err != nil {
		return err
	}
	return nil
}

func (vm *VM) StateSyncEnabled(ctx context.Context) (bool, error) {
	return vm.StateSyncClient.StateSyncEnabled(ctx)
}

func (vm *VM) GetOngoingSyncStateSummary(ctx context.Context) (block.StateSummary, error) {
	return vm.StateSyncClient.GetOngoingSyncStateSummary(ctx)
}

func (vm *VM) ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error) {
	return vm.StateSyncClient.ParseStateSummary(ctx, bytes)
}

func (vm *VM) GetLastStateSummary(ctx context.Context) (block.StateSummary, error) {
	return vm.StateSyncServer.GetLastStateSummary(ctx)
}

func (vm *VM) GetStateSummary(ctx context.Context, height uint64) (block.StateSummary, error) {
	return vm.StateSyncServer.GetStateSummary(ctx, height)
}
