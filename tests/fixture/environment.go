// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"strconv"

	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/tests/workload"
)

func NewTestEnvironment(
	testContext tests.TestContext,
	flagVars *e2e.FlagVars,
	owner string,
	networkConfig workload.TestNetworkConfiguration,
	vmID ids.ID,
) *e2e.TestEnvironment {
	// Run only once in the first ginkgo process
	nodeCount, err := flagVars.NodeCount()
	require.NoError(testContext, err)
	nodes := tmpnet.NewNodesOrPanic(nodeCount)
	nodes[0].Flags[config.HTTPPortKey] = strconv.Itoa(config.DefaultHTTPPort)

	subnet := NewHyperVMSubnet(
		networkConfig.Name(),
		vmID,
		networkConfig.GenesisBytes(),
		nodes...,
	)
	desiredNetwork := NewTmpnetNetwork(owner, nodes, subnet)

	testEnv := e2e.NewTestEnvironment(
		testContext,
		flagVars,
		desiredNetwork,
	)
	network := testEnv.GetNetwork()

	// Create a chain alias on all nodes
	chainID := network.GetSubnet(networkConfig.Name()).Chains[0].ChainID
	for _, node := range network.Nodes {
		setupDefaultChainAlias(testContext, node, chainID, networkConfig.Name())
	}

	return testEnv
}

func setupDefaultChainAlias(tc tests.TestContext, node *tmpnet.Node, chainID ids.ID, vmName string) {
	require := require.New(tc)

	uri, cancel, err := node.GetLocalURI(tc.DefaultContext())
	require.NoError(err)
	defer cancel()

	adminClient := admin.NewClient(uri)

	aliases, err := adminClient.GetChainAliases(tc.DefaultContext(), chainID.String())
	require.NoError(err)

	for _, alias := range aliases {
		if alias == vmName {
			return // already exists
		}
	}

	tc.Log().Info("setting chain alias",
		zap.Stringer("nodeID", node.NodeID),
		zap.String("chainID", chainID.String()),
		zap.String("alias", vmName),
	)
	err = adminClient.AliasChain(tc.DefaultContext(), chainID.String(), vmName)
	require.NoError(err)
}
