// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/utils"

	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "hypersdk-e2e"

var (
	vmName            string
	vmID              ids.ID
	genesisBytes      []byte
	txWorkloadFactory workload.TxWorkloadFactory
	networkID         uint32
	blockchainID      ids.ID
	flagVars          *e2e.FlagVars
)

func init() {
	flagVars = e2e.RegisterFlags()
}

func SetupTestNetwork(
	name string,
	id ids.ID,
	g []byte,
	factory workload.TxWorkloadFactory,
) {
	vmName = name
	vmID = id
	genesisBytes = g
	txWorkloadFactory = factory
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	// Run only once in the first ginkgo process
	nodes := tmpnet.NewNodesOrPanic(flagVars.NodeCount())
	subnet := fixture.NewHyperVMSubnet(
		vmName,
		vmID,
		genesisBytes,
		nodes...,
	)
	network := fixture.NewTmpnetNetwork(owner, nodes, subnet)
	return e2e.NewTestEnvironment(
		e2e.NewTestContext(),
		flagVars,
		network,
	).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
	networkID = e2e.GetEnv(e2e.NewTestContext()).GetNetwork().NetworkID
	blockchainID = e2e.GetEnv(e2e.NewTestContext()).GetNetwork().GetSubnet(vmName).Chains[0].ChainID
})

var _ = ginkgo.Describe("[HyperSDK APIs]", func() {
	tc := e2e.NewTestContext()
	require := require.New(tc)

	ginkgo.It("Ping", func() {
		workload.Ping(tc.DefaultContext(), require, getE2EURIs(tc))
	})

	ginkgo.It("GetNetwork", func() {
		workload.GetNetwork(tc.DefaultContext(), require, getE2EURIs(tc), networkID, blockchainID)
	})
})

var _ = ginkgo.Describe("[HyperSDK Basic Tx Workload]", func() {
	ginkgo.It("Basic Tx Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)

		txs, err := txWorkloadFactory.NewBasicTxWorkload(getE2EURIs(tc)[0])
		require.NoError(err)
		workload.ExecuteWorkload(tc.DefaultContext(), require, getE2EURIs(tc), txs)
	})
})

var _ = ginkgo.Describe("[HyperSDK Syncing]", func() {
	tc := e2e.NewTestContext()
	require := require.New(tc)

	uris := getE2EURIs(tc)
	ginkgo.It("Generate 128 blocks", func() {
		workload.GenerateNBlocks(tc.DefaultContext(), require, uris, txWorkloadFactory, 128)
	})

	var (
		bootstrapNode    *tmpnet.Node
		bootstrapNodeURI string
	)
	ginkgo.It("Start a new node to bootstrap", func() {
		bootstrapNode = bootstrapNewHyperNode(tc)
		bootstrapNodeURI = formatURI(bootstrapNode.URI, blockchainID)
		uris = append(uris, bootstrapNodeURI)
	})
	ginkgo.It("Accept a transaction after state sync", func() {
		txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(bootstrapNodeURI, 1)
		require.NoError(err)
		workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
	})
	ginkgo.It("Restart the node", func() {
		require.NoError(e2e.GetEnv(tc).GetNetwork().RestartNode(tc.DefaultContext(), ginkgo.GinkgoWriter, bootstrapNode))
	})
	ginkgo.It("Generate 1024 blocks", func() {
		workload.GenerateNBlocks(tc.DefaultContext(), require, uris, txWorkloadFactory, 1024)
	})
	var (
		syncNode    *tmpnet.Node
		syncNodeURI string
	)
	ginkgo.It("Start a new node to state sync", func() {
		syncNode = bootstrapNewHyperNode(tc)
		syncNodeURI = formatURI(syncNode.URI, blockchainID)
		uris = append(uris, syncNodeURI)
		utils.Outf("{{blue}}sync node uri: %s{{/}}\n", syncNodeURI)
		c := rpc.NewJSONRPCClient(syncNodeURI)
		_, _, _, err := c.Network(tc.DefaultContext())
		require.NoError(err)
	})
	ginkgo.It("Accept a transaction after state sync", func() {
		txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(syncNodeURI, 1)
		require.NoError(err)
		workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
	})
	ginkgo.It("Pause the node", func() {
		// TODO: can we remove the need to call SaveAPIPort from the test
		require.NoError(syncNode.SaveAPIPort())
		require.NoError(syncNode.Stop(tc.DefaultContext()))

		// TODO: remove extra Ping check and rely on tmpnet to stop the node correctly
		c := rpc.NewJSONRPCClient(syncNodeURI)
		ok, err := c.Ping(tc.DefaultContext())
		require.Error(err) //nolint:forbidigo
		require.False(ok)
	})
	ginkgo.It("Generate 256 blocks", func() {
		// Generate blocks on all nodes except the paused node
		runningURIs := uris[:len(uris)-1]
		workload.GenerateNBlocks(tc.DefaultContext(), require, runningURIs, txWorkloadFactory, 256)
	})
	ginkgo.It("Resume the node", func() {
		require.NoError(e2e.GetEnv(tc).GetNetwork().StartNode(tc.DefaultContext(), ginkgo.GinkgoWriter, syncNode))
		utils.Outf("Waiting for sync node to restart")
		require.NoError(tmpnet.WaitForHealthy(tc.DefaultContext(), syncNode))

		utils.Outf("{{blue}}sync node reporting healthy: %s{{/}}\n", syncNodeURI)

		c := rpc.NewJSONRPCClient(syncNodeURI)
		_, _, _, err := c.Network(tc.DefaultContext())
		require.NoError(err)
	})

	ginkgo.It("Accept a transaction after resuming", func() {
		txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(syncNodeURI, 1)
		require.NoError(err)
		workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
	})
	ginkgo.It("State sync while broadcasting txs", func() {
		ctx, cancel := context.WithCancel(tc.DefaultContext())
		go func() {
			// Recover failure if exits
			defer ginkgo.GinkgoRecover()

			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(uris[0], 128)
			require.NoError(err)
			workload.GenerateUntilCancel(tc.DefaultContext(), uris, txWorkload)
		}()

		// Give time for transactions to start processing
		time.Sleep(5 * time.Second)

		syncConcurrentNode := bootstrapNewHyperNode(tc)
		syncConcurrentNodeURI := formatURI(syncConcurrentNode.URI, blockchainID)
		uris = append(uris, syncConcurrentNodeURI)
		c := rpc.NewJSONRPCClient(syncConcurrentNodeURI)
		_, _, _, err := c.Network(ctx)
		require.NoError(err)
		cancel()
	})
	ginkgo.It("Accept a transaction after syncing", func() {
		txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(uris[0], 1)
		require.NoError(err)
		workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
	})
})

func getE2EURIs(tc tests.TestContext) []string {
	return utils.Map(func(nodeURI tmpnet.NodeURI) string { return nodeURI.URI }, e2e.GetEnv(tc).GetNetwork().GetNodeURIs())
}

func bootstrapNewHyperNode(tc tests.TestContext) *tmpnet.Node {
	// TODO: return the node from CheckBoostrapIsPossible after https://github.com/ava-labs/avalanchego/pull/3253
	e2e.CheckBootstrapIsPossible(tc, e2e.GetEnv(tc).GetNetwork())
	var t *tmpnet.Node
	return t
}

func formatURI(baseURI string, blockchainID ids.ID) string {
	return fmt.Sprintf("%s/ext/bc/%s", baseURI, blockchainID)
}
