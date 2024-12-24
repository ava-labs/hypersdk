// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	hcontext "github.com/ava-labs/hypersdk/context"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/ava-labs/hypersdk/storage"
)

const StateSyncNamespace = "statesync"

type validityWindowAdapter struct {
	*validitywindow.Syncer[*chain.Transaction]
}

type StateSyncConfig struct {
	MerkleSimultaneousWorkLimit int    `json:"merkleSimultaneousWorkLimit"`
	MinBlocks                   uint64 `json:"minBlocks"`
}

func NewDefaultStateSyncConfig() StateSyncConfig {
	return StateSyncConfig{
		MerkleSimultaneousWorkLimit: 4,
		MinBlocks:                   768,
	}
}

func (v validityWindowAdapter) Accept(ctx context.Context, blk *snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]) (bool, error) {
	return v.Syncer.Accept(ctx, blk.Input)
}

func (vm *VM) initStateSync(ctx context.Context) error {
	stateSyncConfig, err := hcontext.GetConfigFromContext(vm.snowInput.Context, StateSyncNamespace, NewDefaultStateSyncConfig())
	if err != nil {
		return err
	}
	stateSyncRegistry, err := vm.snowInput.Context.MakeRegistry("statesync")
	if err != nil {
		return err
	}
	if err := statesync.RegisterHandlers(
		vm.snowCtx.Log,
		vm.network,
		rangeProofHandlerID,
		changeProofHandlerID,
		vm.stateDB,
	); err != nil {
		return err
	}

	vm.syncer = validitywindow.NewSyncer(vm, vm.chainTimeValidityWindow, func(time int64) int64 {
		return vm.ruleFactory.GetRules(time).GetValidityWindow()
	})
	// blockWindowSyncer := statesync.NewBlockWindowSyncer[*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]](validityWindowAdapter{vm.syncer})

	merkleSyncer, err := statesync.NewMerkleSyncer[*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]](
		vm.snowCtx.Log,
		vm.stateDB,
		vm.network,
		rangeProofHandlerID,
		changeProofHandlerID,
		vm.genesis.GetStateBranchFactor(),
		stateSyncConfig.MerkleSimultaneousWorkLimit,
		stateSyncRegistry,
	)
	if err != nil {
		return err
	}

	pebbleConfig := pebble.NewDefaultConfig()
	syncerDB, err := storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, syncerDB, stateSyncRegistry)
	if err != nil {
		return err
	}
	vm.snowApp.WithCloser(func() error {
		if err := syncerDB.Close(); err != nil {
			return fmt.Errorf("failed to close syncer db: %w", err)
		}
		return nil
	})

	covariantVM := vm.snowApp.GetCovariantVM()
	client, err := statesync.NewAggregateClient[*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]](
		vm.snowCtx.Log,
		covariantVM,
		syncerDB,
		[]statesync.Syncer[*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]]{
			// blockWindowSyncer,
			merkleSyncer,
		},
		vm.snowApp.StartStateSync,
		func(ctx context.Context) error {
			outputBlock, err := vm.extractLatestOutputBlock(ctx)
			if err != nil {
				return fmt.Errorf("failed to extract latest output block while finishing state sync: %w", err)
			}
			return vm.snowApp.FinishStateSync(ctx, outputBlock.ExecutionBlock, outputBlock, outputBlock)
		},
		stateSyncConfig.MinBlocks,
	)
	if err != nil {
		return err
	}
	server := statesync.NewServer[*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]](vm.snowCtx.Log, covariantVM)
	stateSyncableVM := statesync.NewStateSyncableVM(client, server)
	vm.snowApp.WithStateSyncableVM(stateSyncableVM)
	return nil
}
