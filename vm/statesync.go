// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/pebble"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/statesync"
	"github.com/ava-labs/hypersdk/storage"

	hcontext "github.com/ava-labs/hypersdk/context"
)

const StateSyncNamespace = "statesync"

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

func GetStateSyncConfig(config hcontext.Config) (StateSyncConfig, error) {
	return hcontext.GetConfig(config, StateSyncNamespace, NewDefaultStateSyncConfig())
}

func (vm *VM) initStateSync(ctx context.Context) error {
	stateSyncConfig, err := GetStateSyncConfig(vm.snowInput.Config)
	if err != nil {
		return err
	}
	stateSyncRegistry, err := metrics.MakeAndRegister(vm.snowCtx.Metrics, StateSyncNamespace)
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

	if err := vm.network.AddHandler(
		blockFetchHandleID,
		validitywindow.NewBlockFetcherHandler[*chain.ExecutionBlock](vm.chainStore)); err != nil {
		return err
	}

	blockFetcherClient := validitywindow.NewBlockFetcherClient[*chain.ExecutionBlock](
		validitywindow.NewP2PBlockFetcher(vm.network.NewClient(blockFetchHandleID)),
		vm,
		p2p.PeerSampler{Peers: vm.network.Peers},
	)
	syncer := validitywindow.NewSyncer[*chain.Transaction, *chain.ExecutionBlock](vm, vm.chainTimeValidityWindow, blockFetcherClient, func(time int64) int64 {
		return vm.ruleFactory.GetRules(time).GetValidityWindow()
	})

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
	pebbleConfig := pebble.NewDefaultConfig()
	syncerDB, err := storage.New(pebbleConfig, vm.snowCtx.ChainDataDir, syncerDB, stateSyncRegistry)
	if err != nil {
		return err
	}
	vm.snowApp.AddCloser("syncer", func() error {
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
			syncer,
			merkleSyncer,
		},
		vm.snowApp.StartStateSync,
		func(ctx context.Context) error {
			vm.snowCtx.Log.Info("State sync completed, extracting the final target state sync block")
			stateHeight, err := vm.extractStateHeight()
			if err != nil {
				return fmt.Errorf("failed to extract state height after state sync: %w", err)
			}
			block, err := vm.chainStore.GetBlockByHeight(ctx, stateHeight+1)
			if err != nil {
				return fmt.Errorf("failed to get block by height %d after state sync: %w", stateHeight, err)
			}
			// Execute at least one block after state sync to ensure ExecutionResults is populated for the last
			// accepted block.
			outputBlock, err := vm.chain.Execute(ctx, vm.stateDB, block, false)
			if err != nil {
				return fmt.Errorf("failed to execute final block %s after state sync: %w", block, err)
			}
			if _, err := vm.AcceptBlock(ctx, nil, outputBlock); err != nil {
				return fmt.Errorf("failed to commit final block %s after state sync: %w", block, err)
			}
			if err := vm.snowApp.FinishStateSync(ctx, outputBlock.ExecutionBlock, outputBlock, outputBlock); err != nil {
				return err
			}
			vm.snowInput.ToEngine <- common.StateSyncDone
			return vm.startNormalOp(ctx)
		},
		stateSyncConfig.MinBlocks,
	)
	if err != nil {
		return err
	}
	vm.snowApp.AddPreReadyAcceptedSub(event.SubscriptionFunc[*chain.ExecutionBlock]{
		NotifyF: func(_ context.Context, b *chain.ExecutionBlock) error {
			return client.UpdateSyncTarget(ctx, b)
		},
	})
	vm.SyncClient = client
	server := statesync.NewServer[*chain.ExecutionBlock](vm.snowCtx.Log, inputCovariantVM)
	stateSyncableVM := statesync.NewStateSyncableVM(client, server)
	vm.snowApp.SetStateSyncableVM(stateSyncableVM)
	return nil
}
