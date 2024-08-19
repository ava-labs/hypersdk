// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/tests/fixture"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "morpheusvm-e2e-tests"

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single MorpheusVM Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	gen, workloadFactory, err := workload.New(100 /* minBlockGap: 100ms */)
	require.NoError(err)

	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject MorpheusVM name
	// and workload factory to orchestrate the test.
	he2e.SetWorkload(consts.Name, workloadFactory)

	// Run only once in the first ginkgo process
	nodes := tmpnet.NewNodesOrPanic(flagVars.NodeCount())

	nodes[0].Flags["http-port"] = "9650"

	subnet := fixture.NewHyperVMSubnet(
		consts.Name,
		consts.ID,
		genesisBytes,
		nodes...,
	)

	network := fixture.NewTmpnetNetwork(owner, nodes, subnet)
	tc := e2e.NewTestContext()
	testEnv := e2e.NewTestEnvironment(
		tc,
		flagVars,
		network,
	)

	he2e.SetupDefaultChainAlias(network.GetSubnet(consts.Name).Chains[0].ChainID, tc)

	return testEnv.Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})

var _ = ginkgo.Describe("[MorpheusVM]", func() {
	ginkgo.It("responds with a valid ABI", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		network := e2e.GetEnv(tc).GetNetwork()
		nodeBaseURI := network.GetNodeURIs()[0].URI
		blockchainID := network.GetSubnet(consts.Name).Chains[0].ChainID
		nodeURI := fmt.Sprintf("%s/ext/bc/%s", nodeBaseURI, blockchainID)

		morpheusRPCClient := rpc.NewJSONRPCClient(nodeURI, network.NetworkID, blockchainID)

		abi, err := morpheusRPCClient.GetABI(context.TODO())
		require.NoError(err)

		var abiJSON []map[string]interface{}
		err = json.Unmarshal([]byte(abi), &abiJSON)
		require.NoError(err)

		obj := abiJSON[0]
		require.Equal(obj["id"], float64(0)) // JSON numbers are parsed as float64
		require.Equal(obj["name"], "Transfer")
	})
})
