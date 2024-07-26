// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/vm"

	avametrics "github.com/ava-labs/avalanchego/api/metrics"
)

var (
	_ vm.Controller        = (*Controller)(nil)
	_ vm.ControllerFactory = (*factory)(nil)
)

var Version = &version.Semantic{
	Major: 0,
	Minor: 0,
	Patch: 1,
}

type Controller struct{}

func (c *Controller) Rules(t int64) chain.Rules {
	return nil
}

func (c *Controller) StateManager() chain.StateManager {
	return nil
}

func (c *Controller) Accepted(ctx context.Context, blk *chain.StatelessBlock) error {
	return nil
}

func (c *Controller) Shutdown(context.Context) error {
	return nil
}

type factory struct{}

func (*factory) New(
	inner *vm.VM,
	log logging.Logger,
	networkID uint32,
	chainID ids.ID,
	chainDataDir string,
	gatherer avametrics.MultiGatherer,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
) (
	Controller,
	vm.Genesis,
	Builder,
	gossiper.Gossiper,
	vm.Handlers,
	chain.ActionRegistry,
	chain.AuthRegistry,
	map[uint8]vm.AuthEngine,
	error,
) {
	return nil, nil, nil, nil, nil, nil, nil, nil, nil
}

func TestNextTimeAdvances(t *testing.T) {
	vm := vm.New(&factory{}, Version)
	time := NewTime(vm)
	time.nextTime(0, 0)
}
