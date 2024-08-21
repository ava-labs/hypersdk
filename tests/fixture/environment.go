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
	"github.com/onsi/ginkgo/v2"
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
	nodes[0].Flags[config.HTTPPortKey] = config.DefaultHTTPPort

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

	chainID := network.GetSubnet(vmName).Chains[0].ChainID
	if chainID != ids.Empty {
		setupDefaultChainAlias(chainID, testContext, vmName)
	}

	return testEnv
}

func setupDefaultChainAlias(chainID ids.ID, tc tests.TestContext, vmName string) {
	require := require.New(ginkgo.GinkgoT())

	baseURI := fmt.Sprintf("http://localhost:%d", config.DefaultHTTPPort)
	adminClient := admin.NewClient(baseURI)

	aliases, err := adminClient.GetChainAliases(tc.DefaultContext(), chainID.String())
	require.NoError(err)

	hasAlias := false
	for _, alias := range aliases {
		if alias == vmName {
			hasAlias = true
		}
	}

	if !hasAlias {
		err = adminClient.AliasChain(tc.DefaultContext(), chainID.String(), vmName)
		require.NoError(err)
	}
}
