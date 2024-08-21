// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"
	"net/http"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/registry"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/extension/indexer"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"

	ametrics "github.com/ava-labs/avalanchego/api/metrics"
	hrpc "github.com/ava-labs/hypersdk/rpc"
	hstorage "github.com/ava-labs/hypersdk/storage"
)

var (
	_ vm.Controller        = (*Controller)(nil)
	_ vm.ControllerFactory = (*factory)(nil)
)

type allocationManager struct{}

func (allocationManager) SetBalance(ctx context.Context, mu state.Mutable, addr codec.Address, balance uint64) error {
	return storage.SetBalance(ctx, mu, addr, balance)
}

func New(options ...vm.Option) (*vm.VM, error) {
	return vm.New(
		&factory{},
		consts.Version,
		registry.Action,
		registry.Auth,
		auth.Engines(),
		&allocationManager{},
		options...,
	)
}

type factory struct{}

func (*factory) New(
	inner *vm.VM,
	log logging.Logger,
	chainDataDir string,
	gatherer ametrics.MultiGatherer,
	configBytes []byte,
) (
	vm.Controller,
	vm.Handlers,
	error,
) {
	c := &Controller{}
	c.inner = inner
	c.log = log
	c.stateManager = &storage.StateManager{}

	// Instantiate metrics
	var err error
	c.metrics, err = newMetrics(gatherer)
	if err != nil {
		return nil, nil, err
	}

	// Load config and genesis
	c.config, err = newConfig(configBytes)
	if err != nil {
		return nil, nil, err
	}

	log.Info("initialized config", zap.Any("contents", c.config))

	c.txDB, err = hstorage.New(pebble.NewDefaultConfig(), chainDataDir, "db", gatherer)
	if err != nil {
		return nil, nil, err
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
		rpc.NewJSONRPCServer(c, inner.Genesis().(*vm.Bech32Genesis), inner.RuleFactory()),
	)
	if err != nil {
		return nil, nil, err
	}
	apis[rpc.JSONRPCEndpoint] = jsonRPCHandler

	return c, apis, nil
}

type Controller struct {
	inner        *vm.VM
	log          logging.Logger
	config       *Config
	stateManager *storage.StateManager

	metrics *metrics

	txDB               database.Database
	txIndexer          indexer.TxIndexer
	acceptedSubscriber indexer.AcceptedSubscriber
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
