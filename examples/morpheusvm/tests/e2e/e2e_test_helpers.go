// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/tests/fixture"
)

// Creates and start a new Morpheus network
// Closing server is responsibility of caller
func NewMorpheusNetwork(numOfNodes int,
	devNetPort string,
	genesisBytes []byte,
	owner string,
	avalancheGoPath string,
	avalancheGoPluginDir string,
) (*tmpnet.Network, error) {
	nodes := tmpnet.NewNodesOrPanic(numOfNodes)
	// Set static port for DevNet
	nodes[0].Flags["http-port"] = devNetPort
	subnet := fixture.NewHyperVMSubnet(
		consts.Name,
		consts.ID,
		genesisBytes,
		nodes...,
	)

	timeOut := 2 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()

	network := fixture.NewTmpnetNetwork(owner, nodes, subnet)
	if err := tmpnet.BootstrapNewNetwork(
		ctx,
		os.Stdout,
		network,
		"",
		avalancheGoPath,
		avalancheGoPluginDir,
	); err != nil {
		return nil, err
	}

	return network, nil
}
