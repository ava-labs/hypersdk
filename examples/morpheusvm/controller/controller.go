// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

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
	"github.com/ava-labs/hypersdk/examples/morpheusvm/config"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/version"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/vm"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	hrpc "github.com/ava-labs/hypersdk/rpc"
	hstorage "github.com/ava-labs/hypersdk/storage"
)

var (
	_ vm.Controller        = (*Controller)(nil)
	_ vm.ControllerFactory = (*factory)(nil)
)

func New() *vm.VM {
	return vm.New(&factory{}, version.Version)
}

type factory struct{}

func (*factory) New(
	inner *vm.VM,
	snowCtx *snow.Context,
	gatherer ametrics.MultiGatherer,
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	vm.Controller,
	vm.Genesis,
	builder.Builder,
	gossiper.Gossiper,
	vm.Handlers,
	chain.ActionRegistry,
	chain.AuthRegistry,
	map[uint8]vm.AuthEngine,
	error,
) {
	c := &Controller{}
	c.inner = inner
	c.snowCtx = snowCtx
	c.stateManager = &storage.StateManager{}

	// Instantiate metrics
	var err error
	c.metrics, err = newMetrics(gatherer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	// Load config and genesis
	c.config, err = config.New(configBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	c.snowCtx.Log.SetLevel(c.config.LogLevel)
	snowCtx.Log.Info("initialized config", zap.Any("contents", c.config))

	c.genesis, err = genesis.New(genesisBytes, upgradeBytes)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf(
			"unable to read genesis: %w",
			err,
		)
	}
	snowCtx.Log.Info("loaded genesis", zap.Any("genesis", c.genesis))

	c.txDB, err = hstorage.New(pebble.NewDefaultConfig(), snowCtx.ChainDataDir, "db", gatherer)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	acceptedSubscribers := []indexer.AcceptedSubscriber{
		indexer.NewSuccessfulTxSubscriber(&actionHandler{c: c}),
	}
	if c.config.StoreTransactions {
		c.txIndexer = indexer.NewTxDBIndexer(c.txDB)
		acceptedSubscribers = append(acceptedSubscribers, c.txIndexer)
	} else {
		c.txIndexer = indexer.NewNoOpTxIndexer()
	}
	c.acceptedSubscriber = indexer.NewAcceptedSubscribers(acceptedSubscribers...)

	// Create handlers
	//
	// hypersdk handler are initiatlized automatically, you just need to
	// initialize custom handlers here.
	apis := map[string]http.Handler{}
	jsonRPCHandler, err := hrpc.NewJSONRPCHandler(
		consts.Name,
		rpc.NewJSONRPCServer(c),
	)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	apis[rpc.JSONRPCEndpoint] = jsonRPCHandler

	// Create builder and gossiper
	build := builder.NewTime(inner)
	gcfg := gossiper.DefaultProposerConfig()
	gossip, err := gossiper.NewProposer(inner, gcfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	return c, c.genesis, build, gossip, apis, consts.ActionRegistry, consts.AuthRegistry, auth.Engines(), nil
}

type Controller struct {
	inner *vm.VM

	snowCtx      *snow.Context
	genesis      *genesis.Genesis
	config       *config.Config
	stateManager *storage.StateManager

	metrics *metrics

	txDB               database.Database
	txIndexer          indexer.TxIndexer
	acceptedSubscriber indexer.AcceptedSubscriber
}

func (c *Controller) Rules(t int64) chain.Rules {
	// TODO: extend with [UpgradeBytes]
	return c.genesis.Rules(t, c.snowCtx.NetworkID, c.snowCtx.ChainID)
}

func (c *Controller) StateManager() chain.StateManager {
	return c.stateManager
}

func (c *Controller) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	return c.acceptedSubscriber.Accepted(ctx, blk)
}

func (c *Controller) Shutdown(context.Context) error {
	// Close any databases created during initialization
	return c.txDB.Close()
}
