// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/config"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/version"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/pebble"
	hypervm "github.com/ava-labs/hypersdk/vm"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"

	hrpc "github.com/ava-labs/hypersdk/rpc"
	hstorage "github.com/ava-labs/hypersdk/storage"
)

var _ hypervm.Interface = (*VM)(nil)

type VM struct {
	inner *hypervm.VM

	snowCtx      *snow.Context
	genesis      *genesis.Genesis
	config       *config.Config
	stateManager *storage.StateManager

	metrics *metrics

	db database.Database
}

func New() *hypervm.VM {
	return hypervm.New(&VM{}, version.Version)
}

func (vm *VM) Initialize(
	inner *hypervm.VM,
	snowCtx *snow.Context,
	gatherer ametrics.MultiGatherer,
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	hypervm.Config,
	hypervm.Genesis,
	builder.Builder,
	gossiper.Gossiper,
	hypervm.Handlers,
	chain.ActionRegistry,
	chain.AuthRegistry,
	map[uint8]hypervm.AuthEngine,
	error,
) {
	vm.inner = inner
	vm.snowCtx = snowCtx
	vm.stateManager = &storage.StateManager{}

	// Instantiate metrics
	var err error
	vm.metrics, err = newMetrics(gatherer)
	if err != nil {
		return hypervm.Config{}, nil, nil, nil, nil, nil, nil, nil, err
	}

	// Load config and genesis
	vm.config, err = config.New(vm.snowCtx.NodeID, configBytes)
	if err != nil {
		return hypervm.Config{}, nil, nil, nil, nil, nil, nil, nil, err
	}
	vm.snowCtx.Log.SetLevel(vm.config.GetLogLevel())
	snowCtx.Log.Info("initialized config", zap.Bool("loaded", vm.config.Loaded()), zap.Any("contents", vm.config))

	vm.genesis, err = genesis.New(genesisBytes, upgradeBytes)
	if err != nil {
		return hypervm.Config{}, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf(
			"unable to read genesis: %w",
			err,
		)
	}
	snowCtx.Log.Info("loaded genesis", zap.Any("genesis", vm.genesis))

	vm.db, err = hstorage.New(pebble.NewDefaultConfig(), snowCtx.ChainDataDir, "db", gatherer)
	if err != nil {
		return hypervm.Config{}, nil, nil, nil, nil, nil, nil, nil, err
	}

	// Create handlers
	//
	// hypersdk handler are initiatlized automatically, you just need to
	// initialize custom handlers here.
	apis := map[string]http.Handler{}
	jsonRPCHandler, err := hrpc.NewJSONRPCHandler(
		consts.Name,
		rpc.NewJSONRPCServer(vm),
	)
	if err != nil {
		return hypervm.Config{}, nil, nil, nil, nil, nil, nil, nil, err
	}
	apis[rpc.JSONRPCEndpoint] = jsonRPCHandler

	// Create builder and gossiper
	var (
		build  builder.Builder
		gossip gossiper.Gossiper
	)
	if vm.config.TestMode {
		vm.inner.Logger().Info("running build and gossip in test mode")
		build = builder.NewManual(inner)
		gossip = gossiper.NewManual(inner)
	} else {
		build = builder.NewTime(inner)
		gcfg := gossiper.DefaultProposerConfig()
		gossip, err = gossiper.NewProposer(inner, gcfg)
		if err != nil {
			return hypervm.Config{}, nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	return vm.config.Config, vm.genesis, build, gossip, apis, consts.ActionRegistry, consts.AuthRegistry, auth.Engines(), nil
}

func (vm *VM) Rules(t int64) chain.Rules {
	// TODO: extend with [UpgradeBytes]
	return vm.genesis.Rules(t, vm.snowCtx.NetworkID, vm.snowCtx.ChainID)
}

func (vm *VM) StateManager() chain.StateManager {
	return vm.stateManager
}

func (vm *VM) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	batch := vm.db.NewBatch()
	defer batch.Reset()

	results := blk.Results()
	for i, tx := range blk.Txs {
		result := results[i]
		if vm.config.GetStoreTransactions() {
			err := storage.StoreTransaction(
				ctx,
				batch,
				tx.ID(),
				blk.GetTimestamp(),
				result.Success,
				result.Units,
				result.Fee,
			)
			if err != nil {
				return err
			}
		}
		if result.Success {
			for _, action := range tx.Actions {
				switch action.(type) { //nolint:gocritic
				case *actions.Transfer:
					vm.metrics.transfer.Inc()
				}
			}
		}
	}
	return batch.Write()
}

func (*VM) Shutdown(context.Context) error {
	// Do not close any databases provided during initialization. The VM will
	// close any databases your provided.
	return nil
}
