// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/state"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/registry"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/vm"
)

var (
	_ vm.Controller        = (*Controller)(nil)
	_ vm.ControllerFactory = (*factory)(nil)
)

// New returns a VM with the indexer, websocket, and rpc apis enabled.
func New(options ...vm.Option) *vm.VM {
	opts := []vm.Option{
		indexer.WithIndexer(consts.Name, indexer.Endpoint),
		ws.WithWebsocketAPI(10_000_000),
		vm.WithVMAPIs(jsonrpc.JSONRPCServerFactory{}, state.JSONRPCStateServerFactory{}),
		vm.WithControllerAPIs(&jsonRPCServerFactory{}),
	}

	opts = append(opts, options...)

	return NewWithOptions(opts...)
}

// NewWithOptions returns a VM with the specified options
func NewWithOptions(options ...vm.Option) *vm.VM {
	return vm.New(
		&factory{},
		consts.Version,
		registry.Action,
		registry.Auth,
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
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	vm.Controller,
	vm.Genesis,
	error,
) {
	c := &Controller{}
	c.inner = inner
	c.log = log
	c.networkID = networkID
	c.chainID = chainID
	c.stateManager = &storage.StateManager{}

	var err error

	// Load config and genesis
	c.config, err = newConfig(configBytes)
	if err != nil {
		return nil, nil, err
	}

	log.Info("initialized config", zap.Any("contents", c.config))

	c.genesis, err = genesis.New(genesisBytes, upgradeBytes)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"unable to read genesis: %w",
			err,
		)
	}
	log.Info("loaded genesis", zap.Any("genesis", c.genesis))

	return c, c.genesis, nil
}

type Controller struct {
	inner     *vm.VM
	log       logging.Logger
	networkID uint32
	chainID   ids.ID

	genesis      *genesis.Genesis
	config       *Config
	stateManager *storage.StateManager
}

func (c *Controller) Rules(t int64) chain.Rules {
	// TODO: extend with [UpgradeBytes]
	return c.genesis.Rules(t, c.networkID, c.chainID)
}

func (c *Controller) StateManager() chain.StateManager {
	return c.stateManager
}
