// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/vm"
)

var (
	_ vm.Controller                     = (*Controller)(nil)
	_ vm.ControllerFactory[*Controller] = (*factory)(nil)
)

func New(options ...vm.Option[*Controller]) (*vm.VM[*Controller], error) {
	return vm.New[*Controller](
		&factory{},
		consts.Version,
		consts.ActionRegistry,
		consts.AuthRegistry,
		auth.Engines(),
		options...,
	)
}

type factory struct{}

func (*factory) New(
	inner *vm.VM[*Controller],
	log logging.Logger,
	networkID uint32,
	chainID ids.ID,
	genesisBytes []byte,
	upgradeBytes []byte, // subnets to allow for AWM
	configBytes []byte,
) (
	*Controller,
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
	inner     *vm.VM[*Controller]
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
