// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/ava-labs/hypersdk/storage"

	hcontext "github.com/ava-labs/hypersdk/context"
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
		MinBlocks:                   512,
	}
}

func (v validityWindowAdapter) Accept(ctx context.Context, blk *chain.ExecutionBlock) (bool, error) {
	return v.Syncer.Accept(ctx, blk)
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
	blockWindowSyncer := statesync.NewBlockWindowSyncer[*chain.ExecutionBlock](validityWindowAdapter{vm.syncer})

	merkleSyncer, err := statesync.NewMerkleSyncer[*chain.ExecutionBlock](
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
	// If we accept an initial target of height N and terminate at the same height,
	// then due to deferred root verification, we will actually have the state for block N-1 and
	// not have the block at height N-1. Therefore, we require an offset of 1 to guarantee we have
	// the block of the merkle trie root we are syncing to.
	// XXX: this could be easily solved if the block window syncer fetched the greater of 1 block
	// prior to the initial target or a validity window of blocks from the final target.
	offsetMerkleSyncer, err := statesync.NewMerkleOffsetAdapter[*chain.ExecutionBlock](
		1,
		merkleSyncer,
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

	inputCovariantVM := vm.snowApp.GetInputCovariantVM()
	client, err := statesync.NewAggregateClient[*chain.ExecutionBlock](
		vm.snowCtx.Log,
		inputCovariantVM,
		syncerDB,
		[]statesync.Syncer[*chain.ExecutionBlock]{
			blockWindowSyncer,
			offsetMerkleSyncer,
		},
		vm.snowApp.StartStateSync,
		func(ctx context.Context) error {
			outputBlock, err := vm.extractLatestOutputBlock(ctx)
			if err != nil {
				return fmt.Errorf("failed to extract latest output block while finishing state sync: %w", err)
			}
			if err := vm.snowApp.FinishStateSync(ctx, outputBlock.ExecutionBlock, outputBlock, outputBlock); err != nil {
				return err
			}
			return nil
		},
		stateSyncConfig.MinBlocks,
	)
	if err != nil {
		return err
	}
	vm.snowApp.WithPreReadyAcceptedSub(event.SubscriptionFunc[*chain.ExecutionBlock]{
		NotifyF: func(_ context.Context, b *chain.ExecutionBlock) error {
			return client.UpdateSyncTarget(ctx, b)
		},
	})
	vm.SyncClient = client
	server := statesync.NewServer[*chain.ExecutionBlock](vm.snowCtx.Log, inputCovariantVM)
	stateSyncableVM := statesync.NewStateSyncableVM(client, server)
	vm.snowApp.WithStateSyncableVM(stateSyncableVM)
	return nil
}
