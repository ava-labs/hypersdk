// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/utils"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var (
	vmName            string
	txWorkloadFactory workload.TxWorkloadFactory
)

func SetWorkload(name string, factory workload.TxWorkloadFactory) {
	vmName = name
	txWorkloadFactory = factory
}

var _ = ginkgo.Describe("[HyperSDK APIs]", func() {
	tc := e2e.NewTestContext()
	require := require.New(tc)

	ginkgo.It("Ping", func() {
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(vmName).Chains[0].ChainID
		workload.Ping(tc.DefaultContext(), require, getE2EURIs(tc, expectedBlockchainID))
	})

	ginkgo.It("StableNetworkIdentity", func() {
		hardcodedHostPort := "http://localhost:9650"
		fixedNodeURL := hardcodedHostPort + "/ext/bc/" + vmName

		c := jsonrpc.NewJSONRPCClient(fixedNodeURL)
		_, _, chainIDFromRPC, err := c.Network(tc.DefaultContext())
		require.NoError(err)
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(vmName).Chains[0].ChainID
		require.Equal(expectedBlockchainID, chainIDFromRPC)
	})

	ginkgo.It("GetNetwork", func() {
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(vmName).Chains[0].ChainID
		baseURIs := getE2EBaseURIs(tc)
		baseURI := baseURIs[0]
		client := info.NewClient(baseURI)
		expectedNetworkID, err := client.GetNetworkID(tc.DefaultContext())
		require.NoError(err)
		workload.GetNetwork(tc.DefaultContext(), require, getE2EURIs(tc, expectedBlockchainID), expectedNetworkID, expectedBlockchainID)
	})

	ginkgo.It("GetABI", func() {
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(vmName).Chains[0].ChainID
		workload.GetABI(tc.DefaultContext(), require, getE2EURIs(tc, expectedBlockchainID))
	})
})

var _ = ginkgo.Describe("[HyperSDK Tx Workloads]", func() {
	ginkgo.It("Basic Tx Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(vmName).Chains[0].ChainID

		txWorkloads, err := txWorkloadFactory.NewWorkloads(getE2EURIs(tc, blockchainID)[0])
		require.NoError(err)
		for _, txWorkload := range txWorkloads {
			workload.ExecuteWorkload(tc.DefaultContext(), require, getE2EURIs(tc, blockchainID), txWorkload)
		}
	})
})

var _ = ginkgo.Describe("[HyperSDK Syncing]", func() {
	ginkgo.It("[Sync]", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(vmName).Chains[0].ChainID

		uris := getE2EURIs(tc, blockchainID)
		ginkgo.By("Generate 128 blocks", func() {
			workload.GenerateNBlocks(tc.ContextWithTimeout(5*time.Minute), require, uris, txWorkloadFactory, 128)
		})

		var (
			bootstrapNode    *tmpnet.Node
			bootstrapNodeURI string
		)
		ginkgo.By("Start a new node to bootstrap", func() {
			bootstrapNode = e2e.CheckBootstrapIsPossible(tc, e2e.GetEnv(tc).GetNetwork())
			bootstrapNodeURI = formatURI(bootstrapNode.URI, blockchainID)
			uris = append(uris, bootstrapNodeURI)
		})
		ginkgo.By("Accept a transaction after state sync", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(bootstrapNodeURI, 1)
			require.NoError(err)
			workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
		})
		ginkgo.By("Restart the node", func() {
			require.NoError(e2e.GetEnv(tc).GetNetwork().RestartNode(tc.DefaultContext(), ginkgo.GinkgoWriter, bootstrapNode))
		})
		ginkgo.By("Generate 1024 blocks", func() {
			workload.GenerateNBlocks(tc.ContextWithTimeout(20*time.Minute), require, uris, txWorkloadFactory, 1024)
		})
		var (
			syncNode    *tmpnet.Node
			syncNodeURI string
		)
		ginkgo.By("Start a new node to state sync", func() {
			syncNode = e2e.CheckBootstrapIsPossible(tc, e2e.GetEnv(tc).GetNetwork())
			syncNodeURI = formatURI(syncNode.URI, blockchainID)
			uris = append(uris, syncNodeURI)
			utils.Outf("{{blue}}sync node uri: %s{{/}}\n", syncNodeURI)
			c := jsonrpc.NewJSONRPCClient(syncNodeURI)
			_, _, _, err := c.Network(tc.DefaultContext())
			require.NoError(err)
		})
		ginkgo.By("Accept a transaction after state sync", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(syncNodeURI, 1)
			require.NoError(err)
			workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
		})
		ginkgo.By("Pause the node", func() {
			// TODO: remove the need to call SaveAPIPort from the test
			require.NoError(syncNode.SaveAPIPort())
			require.NoError(syncNode.Stop(tc.DefaultContext()))

			// TODO: remove extra Ping check and rely on tmpnet to stop the node correctly
			c := jsonrpc.NewJSONRPCClient(syncNodeURI)
			ok, err := c.Ping(tc.DefaultContext())
			require.Error(err) //nolint:forbidigo
			require.False(ok)
		})
		ginkgo.By("Generate 256 blocks", func() {
			// Generate blocks on all nodes except the paused node
			runningURIs := uris[:len(uris)-1]
			workload.GenerateNBlocks(tc.ContextWithTimeout(5*time.Minute), require, runningURIs, txWorkloadFactory, 256)
		})
		ginkgo.By("Resume the node", func() {
			require.NoError(e2e.GetEnv(tc).GetNetwork().StartNode(tc.DefaultContext(), ginkgo.GinkgoWriter, syncNode))
			utils.Outf("Waiting for sync node to restart")
			require.NoError(tmpnet.WaitForHealthy(tc.DefaultContext(), syncNode))

			utils.Outf("{{blue}}sync node reporting healthy: %s{{/}}\n", syncNodeURI)

			c := jsonrpc.NewJSONRPCClient(syncNodeURI)
			_, _, _, err := c.Network(tc.DefaultContext())
			require.NoError(err)
		})

		ginkgo.By("Accept a transaction after resuming", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(syncNodeURI, 1)
			require.NoError(err)
			workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
		})
		ginkgo.By("State sync while broadcasting txs", func() {
			ctx, cancel := context.WithCancel(tc.DefaultContext())
			defer cancel()
			go func() {
				// Recover failure if exits
				defer ginkgo.GinkgoRecover()

				txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(uris[0], 128)
				require.NoError(err)
				workload.GenerateUntilCancel(tc.DefaultContext(), uris, txWorkload)
			}()

			// Give time for transactions to start processing
			time.Sleep(5 * time.Second)

			syncConcurrentNode := e2e.CheckBootstrapIsPossible(tc, e2e.GetEnv(tc).GetNetwork())
			syncConcurrentNodeURI := formatURI(syncConcurrentNode.URI, blockchainID)
			uris = append(uris, syncConcurrentNodeURI)
			c := jsonrpc.NewJSONRPCClient(syncConcurrentNodeURI)
			_, _, _, err := c.Network(ctx)
			require.NoError(err)
		})
		ginkgo.By("Accept a transaction after syncing", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(uris[0], 1)
			require.NoError(err)
			workload.ExecuteWorkload(tc.DefaultContext(), require, uris, txWorkload)
		})
	})
})

func getE2EURIs(tc tests.TestContext, blockchainID ids.ID) []string {
	nodeURIs := e2e.GetEnv(tc).GetNetwork().GetNodeURIs()
	uris := make([]string, 0, len(nodeURIs))
	for _, nodeURI := range nodeURIs {
		uris = append(uris, formatURI(nodeURI.URI, blockchainID))
	}
	return uris
}

func getE2EBaseURIs(tc tests.TestContext) []string {
	nodeURIs := e2e.GetEnv(tc).GetNetwork().GetNodeURIs()
	uris := make([]string, 0, len(nodeURIs))
	for _, nodeURI := range nodeURIs {
		uris = append(uris, nodeURI.URI)
	}
	return uris
}

func formatURI(baseURI string, blockchainID ids.ID) string {
	return fmt.Sprintf("%s/ext/bc/%s", baseURI, blockchainID)
}
