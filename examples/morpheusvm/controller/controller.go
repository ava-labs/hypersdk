// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
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
		vm.WithVMAPIs(jsonrpc.JSONRPCServerFactory{}),
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
		append([]vm.Option{
			vm.WithCustomGenesisLoader(
				func(b []byte) (vm.Genesis, error) {
					return vm.LoadBech32Genesis(b, storage.SetBalance)
				},
			),
		}, options...)...,
	)
}

type factory struct{}

func (*factory) New(
	inner *vm.VM,
	log logging.Logger,
	configBytes []byte,
) (
	vm.Controller,
	error,
) {
	c := &Controller{}
	c.inner = inner
	c.log = log
	c.stateManager = &storage.StateManager{}

	var err error
	// Load config and genesis
	c.config, err = newConfig(configBytes)
	if err != nil {
		return nil, err
	}

	log.Info("initialized config", zap.Any("contents", c.config))
	return c, nil
}

type Controller struct {
	inner        *vm.VM
	log          logging.Logger
	config       *Config
	stateManager *storage.StateManager
}

func (c *Controller) StateManager() chain.StateManager {
	return c.stateManager
}
