// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"
	"fmt"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/pebble"
	hrpc "github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/config"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/version"
)

var _ vm.Controller = (*Controller)(nil)

type Controller struct {
	inner *vm.VM

	snowCtx      *snow.Context
	genesis      *genesis.Genesis
	config       *config.Config
	stateManager *storage.StateManager

	metrics *metrics

	metaDB database.Database
}

func New() *vm.VM {
	return vm.New(&Controller{}, version.Version)
}

func (c *Controller) Initialize(
	inner *vm.VM,
	snowCtx *snow.Context,
	gatherer ametrics.MultiGatherer,
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	vm.Config,
	vm.Genesis,
	builder.Builder,
	gossiper.Gossiper,
	database.Database,
	database.Database,
	vm.Handlers,
	chain.ActionRegistry,
	chain.AuthRegistry,
	error,
) {
	c.inner = inner
	c.snowCtx = snowCtx
	c.stateManager = &storage.StateManager{}

	// Instantiate metrics
	var err error
	c.metrics, err = newMetrics(gatherer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	// Load config and genesis
	c.config, err = config.New(c.snowCtx.NodeID, configBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	c.snowCtx.Log.SetLevel(c.config.GetLogLevel())
	snowCtx.Log.Info("loaded config", zap.Any("contents", c.config))

	c.genesis, err = genesis.New(genesisBytes, upgradeBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf(
			"unable to read genesis: %w",
			err,
		)
	}
	snowCtx.Log.Info("loaded genesis", zap.Any("genesis", c.genesis))

	// Create DBs
	blockPath, err := utils.InitSubDirectory(snowCtx.ChainDataDir, "block")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	// TODO: tune Pebble config based on each sub-db focus
	cfg := pebble.NewDefaultConfig()
	blockDB, err := pebble.New(blockPath, cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	statePath, err := utils.InitSubDirectory(snowCtx.ChainDataDir, "state")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	stateDB, err := pebble.New(statePath, cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	metaPath, err := utils.InitSubDirectory(snowCtx.ChainDataDir, "metadata")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	c.metaDB, err = pebble.New(metaPath, cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	// Create handlers
	//
	// hypersdk handler are initiatlized automatically, you just need to
	// initialize custom handlers here.
	apis := map[string]*common.HTTPHandler{}
	jsonRPCHandler, err := hrpc.NewJSONRPCHandler(
		consts.Name,
		rpc.NewJSONRPCServer(c),
		common.NoLock,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	apis[rpc.JSONRPCEndpoint] = jsonRPCHandler

	// Create builder and gossiper
	var (
		build  builder.Builder
		gossip gossiper.Gossiper
	)
	if c.config.TestMode {
		c.inner.Logger().Info("running build and gossip in test mode")
		build = builder.NewManual(inner)
		gossip = gossiper.NewManual(inner)
	} else {
		build = builder.NewTime(inner)
		gcfg := gossiper.DefaultProposerConfig()
		gossip = gossiper.NewProposer(inner, gcfg)
	}
	return c.config, c.genesis, build, gossip, blockDB, stateDB, apis, consts.ActionRegistry, consts.AuthRegistry, nil
}

func (c *Controller) Rules(t int64) chain.Rules {
	// TODO: extend with [UpgradeBytes]
	return c.genesis.Rules(t, c.snowCtx.NetworkID, c.snowCtx.ChainID)
}

func (c *Controller) StateManager() chain.StateManager {
	return c.stateManager
}

func (c *Controller) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	batch := c.metaDB.NewBatch()
	defer batch.Reset()

	results := blk.Results()
	for i, tx := range blk.Txs {
		result := results[i]
		err := storage.StoreTransaction(
			ctx,
			batch,
			tx.ID(),
			blk.GetTimestamp(),
			result.Success,
			result.Units,
		)
		if err != nil {
			return err
		}
		if result.Success {
			switch tx.Action.(type) { //nolint:gocritic
			case *actions.Transfer:
				c.metrics.transfer.Inc()
			}
		}
	}
	return batch.Write()
}

func (*Controller) Rejected(context.Context, *chain.StatelessBlock) error {
	return nil
}

func (*Controller) Shutdown(context.Context) error {
	// Do not close any databases provided during initialization. The VM will
	// close any databases your provided.
	return nil
}
