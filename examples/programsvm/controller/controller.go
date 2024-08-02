// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"context"
	"fmt"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/programsvm/actions"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"net/http"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/programsvm/config"
	"github.com/ava-labs/hypersdk/examples/programsvm/consts"
	"github.com/ava-labs/hypersdk/examples/programsvm/genesis"
	"github.com/ava-labs/hypersdk/examples/programsvm/rpc"
	"github.com/ava-labs/hypersdk/examples/programsvm/storage"
	"github.com/ava-labs/hypersdk/examples/programsvm/version"
	"github.com/ava-labs/hypersdk/extension/indexer"
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

func New(options ...vm.Option) (*vm.VM, error) {
	return vm.New(
		&factory{},
		version.Version,
		consts.ActionRegistry,
		consts.AuthRegistry,
		auth.Engines(),
		options...,
	)
}

type factory struct{}

func (*factory) New(
	inner *vm.VM,
	log logging.Logger,
	networkID uint32,
	chainID ids.ID,
	chainDataDir string,
	gatherer ametrics.MultiGatherer,
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	vm.Controller,
	vm.Genesis,
	vm.Handlers,
	error,
) {
	c := &Controller{}
	c.inner = inner
	c.log = log
	c.networkID = networkID
	c.chainID = chainID
	c.stateManager = &storage.StateManager{}

	// Instantiate metrics
	var err error
	c.metrics, err = newMetrics(gatherer)
	if err != nil {
		return nil, nil, nil, err
	}

	// Load config and genesis
	c.config, err = config.New(configBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	log.Info("initialized config", zap.Any("contents", c.config))

	c.genesis, err = genesis.New(genesisBytes, upgradeBytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"unable to read genesis: %w",
			err,
		)
	}
	log.Info("loaded genesis", zap.Any("genesis", c.genesis))

	c.txDB, err = hstorage.New(pebble.NewDefaultConfig(), chainDataDir, "db", gatherer)
	if err != nil {
		return nil, nil, nil, err
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
		rpc.NewJSONRPCServer(c, c.Simulate),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	apis[rpc.JSONRPCEndpoint] = jsonRPCHandler

	consts.ProgramRuntime = runtime.NewRuntime(runtime.NewConfig(), c.Logger())

	return c, c.genesis, apis, nil
}

type Controller struct {
	inner     *vm.VM
	log       logging.Logger
	networkID uint32
	chainID   ids.ID

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
	return c.genesis.Rules(t, c.networkID, c.chainID)
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

func (c *Controller) Simulate(ctx context.Context, t actions.CallProgram, actor codec.Address) (state.Keys, error) {
	db, err := c.inner.State()
	if err != nil {
		return nil, err
	}
	recorder := &storage.Recorder{State: db}
	_, err = consts.ProgramRuntime.CallProgram(ctx, &runtime.CallInfo{
		Program:      t.Program,
		Actor:        actor,
		State:        &storage.ProgramStateManager{Mutable: recorder},
		FunctionName: t.Function,
		Params:       t.CallData,
		Fuel:         100000000,
	})
	return recorder.GetStateKeys(), err
}
