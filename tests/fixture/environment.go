// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"
)

func NewTestEnvironment(
	testContext tests.TestContext,
	flagVars *e2e.FlagVars,
	owner string,
	vmName string,
	vmID ids.ID,
	genesisBytes []byte,
) *e2e.TestEnvironment {
	// Run only once in the first ginkgo process
	nodes := tmpnet.NewNodesOrPanic(flagVars.NodeCount())

	subnet := NewHyperVMSubnet(
		vmName,
		vmID,
		genesisBytes,
		nodes...,
	)
	network := NewTmpnetNetwork(owner, nodes, subnet)

	testEnv := e2e.NewTestEnvironment(
		testContext,
		flagVars,
		network,
	)

	envNetwork := testEnv.GetNetwork()
	chainID := envNetwork.GetSubnet(vmName).Chains[0].ChainID
	setupDefaultChainAlias(
		testContext,
		envNetwork.Nodes[0].URI,
		chainID,
		vmName,
	)

	return testEnv
}

func setupDefaultChainAlias(tc tests.TestContext, uri string, chainID ids.ID, vmName string) {
	require := require.New(tc)

	adminClient := admin.NewClient(uri)

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
