// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"fmt"

	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/tests/workload"
)

var StableNodeURI = fmt.Sprintf("http://localhost:%d", config.DefaultHTTPPort)

func NewTestEnvironment(
	testContext tests.TestContext,
	flagVars *e2e.FlagVars,
	owner string,
	networkConfig workload.TestNetworkConfiguration,
	vmID ids.ID,
) *e2e.TestEnvironment {
	// Run only once in the first ginkgo process
	nodes := tmpnet.NewNodesOrPanic(flagVars.NodeCount())
	nodes[0].Flags[config.HTTPPortKey] = config.DefaultHTTPPort

	subnet := NewHyperVMSubnet(
		networkConfig.Name(),
		vmID,
		networkConfig.GenesisBytes(),
		nodes...,
	)
	network := NewTmpnetNetwork(owner, nodes, subnet)

	testEnv := e2e.NewTestEnvironment(
		testContext,
		flagVars,
		network,
	)

	chainID := testEnv.GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
	setupDefaultChainAlias(testContext, chainID, networkConfig.Name())

	return testEnv
}

func setupDefaultChainAlias(tc tests.TestContext, chainID ids.ID, vmName string) {
	require := require.New(tc)

	adminClient := admin.NewClient(StableNodeURI)

	aliases, err := adminClient.GetChainAliases(tc.DefaultContext(), chainID.String())
	require.NoError(err)

	for _, alias := range aliases {
		if alias == vmName {
			return // already exists
		}
	}

	err = adminClient.AliasChain(tc.DefaultContext(), chainID.String(), vmName)
	require.NoError(err)
}
