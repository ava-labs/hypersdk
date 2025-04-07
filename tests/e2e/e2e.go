// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/state"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/load"
	"github.com/ava-labs/hypersdk/tests/registry"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/utils"

	ginkgo "github.com/onsi/ginkgo/v2"
)

const (
	metricsURI = "localhost:8080"
	// relative to the user home directory
	metricsFilePath = ".tmpnet/prometheus/file_sd_configs/hypersdk-e2e-metrics.json"
)

var (
	networkConfig workload.TestNetworkConfiguration
	txWorkload    workload.TxWorkload
	expectedABI   abi.ABI

	tracker           load.Tracker[ids.ID]
	loadTxGenerator   LoadTxGenerator
	shortBurstConfig  load.ShortBurstOrchestratorConfig
	gradualLoadConfig load.GradualLoadOrchestratorConfig

	fundDistributor  FundDistributor
	fundConsolidator FundConsolidator
)

// LoadTxGenerator returns the components necessary to instantiate an
// Orchestrator.
// We use a generator here since the node URIs are not known until runtime.
type LoadTxGenerator func(
	ctx context.Context,
	uri string,
	authFactories []chain.AuthFactory,
) ([]load.TxGenerator[*chain.Transaction], error)

// FundDistributor distributes funds from a funder account to a set of test accounts.
type FundDistributor func(context.Context, string, chain.AuthFactory, uint64) ([]chain.AuthFactory, error)

// FundConsolidator collects the funds from the test accounts and sends them
// back to the funder account.
type FundConsolidator func(context.Context, string, []chain.AuthFactory, chain.AuthFactory) error

func SetWorkload(
	networkConfigImpl workload.TestNetworkConfiguration,
	workloadTxGenerator workload.TxGenerator,
	abi abi.ABI,
	generator LoadTxGenerator,
	loadTracker load.Tracker[ids.ID],
	shortBurstConf load.ShortBurstOrchestratorConfig,
	gradualLoadConf load.GradualLoadOrchestratorConfig,
	distributor FundDistributor,
	consolidator FundConsolidator,
) {
	networkConfig = networkConfigImpl
	txWorkload = workload.TxWorkload{Generator: workloadTxGenerator}
	expectedABI = abi
	loadTxGenerator = generator
	tracker = loadTracker
	shortBurstConfig = shortBurstConf
	gradualLoadConfig = gradualLoadConf
	fundDistributor = distributor
	fundConsolidator = consolidator
}

// ExposeMetrics begins a HTTP server that exposes the metrics of registry and
// writes the collector config to a file which tmpnet uses to discover the
// server and scrape its metrics.
// Returns a cleanup function which, when called, stops the server and removes
// the collector config file.
// ExposeMetrics his should be called at most once
func ExposeMetrics(
	ctx context.Context,
	testEnv *e2e.TestEnvironment,
	registry *prometheus.Registry,
) (func() error, error) {
	// Start metrics server
	mux := http.NewServeMux()
	mux.Handle("/ext/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Registry: registry,
	}))

	metricsServer := &http.Server{
		Addr:              metricsURI,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	var metricsServerErr error
	go func() {
		metricsServerErr = metricsServer.ListenAndServe()
	}()

	// Generate collector config
	collectorConfigBytes, err := generateCollectorConfig(
		[]string{metricsURI},
		testEnv.GetNetwork().UUID,
	)
	if err != nil {
		return nil, err
	}

	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(homedir, metricsFilePath)

	if err := writeCollectorConfig(filePath, collectorConfigBytes); err != nil {
		return nil, err
	}

	return func() error {
		if err := metricsServer.Shutdown(ctx); err != nil {
			return err
		}
		if metricsServerErr != http.ErrServerClosed {
			return metricsServerErr
		}
		return os.Remove(filePath)
	}, nil
}

var _ = ginkgo.Describe("[HyperSDK APIs]", func() {
	tc := e2e.NewTestContext()
	require := require.New(tc)

	ginkgo.It("Ping", func() {
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		workload.Ping(tc.DefaultContext(), require, getE2EURIs(tc, expectedBlockchainID))
	})

	ginkgo.It("StableNetworkIdentity", func() {
		hardcodedHostPort := "http://localhost:9650"
		fixedNodeURL := hardcodedHostPort + "/ext/bc/" + networkConfig.Name()

		c := jsonrpc.NewJSONRPCClient(fixedNodeURL)
		_, _, chainIDFromRPC, err := c.Network(tc.DefaultContext())
		require.NoError(err)
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		require.Equal(expectedBlockchainID, chainIDFromRPC)
	})

	ginkgo.It("GetNetwork", func() {
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		baseURIs := getE2EBaseURIs(tc)
		baseURI := baseURIs[0]
		client := info.NewClient(baseURI)
		expectedNetworkID, err := client.GetNetworkID(tc.DefaultContext())
		require.NoError(err)
		workload.GetNetwork(tc.DefaultContext(), require, getE2EURIs(tc, expectedBlockchainID), expectedNetworkID, expectedBlockchainID)
	})

	ginkgo.It("GetABI", func() {
		expectedBlockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		workload.GetABI(tc.DefaultContext(), require, getE2EURIs(tc, expectedBlockchainID), expectedABI)
	})

	ginkgo.It("ReadState", func() {
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		ctx := tc.DefaultContext()
		for _, uri := range getE2EURIs(tc, blockchainID) {
			client := state.NewJSONRPCStateClient(uri)
			values, readerrs, err := client.ReadState(ctx, [][]byte{
				[]byte(`my-unknown-key`),
			})
			require.NoError(err)
			require.Len(values, 1)
			require.Len(readerrs, 1)
		}
	})
})

var _ = ginkgo.Describe("[HyperSDK Tx Workloads]", ginkgo.Serial, func() {
	ginkgo.It("Basic Tx Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID

		ginkgo.By("Tx workloads", func() {
			txWorkload.GenerateBlocks(tc.DefaultContext(), require, getE2EURIs(tc, blockchainID), 1)
		})

		ginkgo.By("Confirm accepted blocks indexed", func() {
			workload.GetBlocks(tc.DefaultContext(), require, networkConfig.Parser(), getE2EURIs(tc, blockchainID))
		})
	})
})

var _ = ginkgo.Describe("[HyperSDK Load Workloads]", ginkgo.Serial, func() {
	ginkgo.It("Short Burst Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		ctx := tc.DefaultContext()
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		uris := getE2EURIs(tc, blockchainID)

		accounts, err := fundDistributor(ctx, uris[0], networkConfig.AuthFactories()[0], uint64(len(uris)))
		require.NoError(err)

		txGenerators, err := loadTxGenerator(
			ctx,
			uris[0],
			accounts,
		)
		require.NoError(err)

		issuers := make([]load.Issuer[*chain.Transaction], len(txGenerators))
		for i := 0; i < len(txGenerators); i++ {
			issuer, err := load.NewDefaultIssuer(uris[i%len(uris)], tracker)
			require.NoError(err)
			issuers[i] = issuer
		}

		orchestrator, err := load.NewShortBurstOrchestrator(
			txGenerators,
			issuers,
			tracker,
			shortBurstConfig,
		)
		require.NoError(err)

		require.NoError(orchestrator.Execute(ctx))

		numTxs := shortBurstConfig.TxsPerIssuer * uint64(len(issuers))
		require.Equal(numTxs, tracker.GetObservedIssued())
		require.Equal(numTxs, tracker.GetObservedConfirmed())
		require.Equal(uint64(0), tracker.GetObservedFailed())

		require.NoError(fundConsolidator(ctx, uris[0], accounts, networkConfig.AuthFactories()[0]))
	})

	ginkgo.It("Gradual Load Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		ctx := tc.ContextWithTimeout(15 * time.Minute)
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		uris := getE2EURIs(tc, blockchainID)

		accounts, err := fundDistributor(ctx, uris[0], networkConfig.AuthFactories()[0], uint64(len(uris)))
		require.NoError(err)

		txGenerators, err := loadTxGenerator(
			ctx,
			uris[0],
			accounts,
		)
		require.NoError(err)

		issuers := make([]load.Issuer[*chain.Transaction], len(txGenerators))
		for i := 0; i < len(txGenerators); i++ {
			issuer, err := load.NewDefaultIssuer(uris[i%len(uris)], tracker)
			require.NoError(err)
			issuers[i] = issuer
		}

		orchestrator, err := load.NewGradualLoadOrchestrator(
			txGenerators,
			issuers,
			tracker,
			tc.Log(),
			gradualLoadConfig,
		)
		require.NoError(err)

		if err := orchestrator.Execute(ctx); err != nil {
			require.ErrorIs(err, context.Canceled)
		}

		require.GreaterOrEqual(tracker.GetObservedIssued(), gradualLoadConfig.MaxTPS)

		require.NoError(fundConsolidator(ctx, uris[0], accounts, networkConfig.AuthFactories()[0]))
	})
})

var _ = ginkgo.Describe("[HyperSDK Syncing]", ginkgo.Serial, func() {
	ginkgo.It("[Sync]", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID

		uris := getE2EURIs(tc, blockchainID)
		ginkgo.By("Generate 32 blocks", func() {
			txWorkload.GenerateBlocks(tc.ContextWithTimeout(5*time.Minute), require, uris, 32)
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
		ginkgo.By("Accept a transaction after bootstrapping", func() {
			txWorkload.GenerateTxs(tc.DefaultContext(), require, 1, bootstrapNodeURI, uris)
		})

		ginkgo.By("Restart the node", func() {
			require.NoError(e2e.GetEnv(tc).GetNetwork().RestartNode(tc.DefaultContext(), tc.Log(), bootstrapNode))
			bootstrapNodeURI = formatURI(bootstrapNode.URI, blockchainID)
			uris[len(uris)-1] = bootstrapNodeURI
		})
		ginkgo.By("Generate > StateSyncMinBlocks=128", func() {
			txWorkload.GenerateBlocks(tc.ContextWithTimeout(20*time.Minute), require, uris, 128)
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
			txWorkload.GenerateTxs(tc.ContextWithTimeout(20*time.Second), require, 1, syncNodeURI, uris)
		})
		ginkgo.By("Pause the node", func() {
			require.NoError(syncNode.Stop(tc.DefaultContext()))

			// TODO: remove extra Ping check and rely on tmpnet to stop the node correctly
			c := jsonrpc.NewJSONRPCClient(syncNodeURI)
			ok, err := c.Ping(tc.ContextWithTimeout(10 * time.Second))
			require.Error(err) //nolint:forbidigo
			require.False(ok)
		})
		ginkgo.By("Generate 32 blocks", func() {
			// Generate blocks on all nodes except the paused node
			runningURIs := uris[:len(uris)-1]
			txWorkload.GenerateBlocks(tc.ContextWithTimeout(5*time.Minute), require, runningURIs, 32)
		})
		ginkgo.By("Resume the node", func() {
			require.NoError(e2e.GetEnv(tc).GetNetwork().StartNode(tc.DefaultContext(), tc.Log(), syncNode))
			utils.Outf("Waiting for sync node to restart")
			require.NoError(tmpnet.WaitForHealthy(tc.DefaultContext(), syncNode))

			syncNodeURI = formatURI(syncNode.URI, blockchainID)
			utils.Outf("{{blue}}sync node reporting healthy: %s{{/}}\n", syncNodeURI)

			c := jsonrpc.NewJSONRPCClient(syncNodeURI)
			_, _, _, err := c.Network(tc.DefaultContext())
			require.NoError(err)

			uris[len(uris)-1] = syncNodeURI
		})

		ginkgo.By("Accept a transaction after resuming", func() {
			txWorkload.GenerateTxs(tc.ContextWithTimeout(20*time.Second), require, 1, syncNodeURI, uris)
		})
		ginkgo.By("State sync while broadcasting txs", func() {
			stopChannel := make(chan struct{})
			wg := &sync.WaitGroup{}
			defer wg.Wait()
			defer close(stopChannel)

			wg.Add(1)
			go func() {
				defer wg.Done()
				// Recover failure if exits
				defer ginkgo.GinkgoRecover()
				txWorkload.GenerateUntilStop(tc.DefaultContext(), require, uris, 128, stopChannel)
			}()

			// Give time for transactions to start processing
			time.Sleep(5 * time.Second)

			syncConcurrentNode := e2e.CheckBootstrapIsPossible(tc, e2e.GetEnv(tc).GetNetwork())
			syncConcurrentNodeURI := formatURI(syncConcurrentNode.URI, blockchainID)
			uris = append(uris, syncConcurrentNodeURI)
			c := jsonrpc.NewJSONRPCClient(syncConcurrentNodeURI)
			_, _, _, err := c.Network(tc.DefaultContext())
			require.NoError(err)
		})
		ginkgo.By("Accept a transaction after syncing", func() {
			txWorkload.GenerateTxs(tc.DefaultContext(), require, 1, uris[0], uris)
		})
	})
})

var _ = ginkgo.Describe("[Custom VM Tests]", ginkgo.Serial, func() {
	tc := e2e.NewTestContext()

	for testRegistry := range registry.GetTestsRegistries() {
		for _, test := range testRegistry.List() {
			ginkgo.It(test.Name, func() {
				testNetwork := NewNetwork(tc)
				test.Fnc(ginkgo.GinkgoT(), testNetwork)
			})
		}
	}
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

func generateCollectorConfig(targets []string, uuid string) ([]byte, error) {
	nodeLabels := tmpnet.FlagsMap{
		"network_owner": "hypersdk-e2e-tests",
		"network_uuid":  uuid,
	}
	config := []tmpnet.FlagsMap{
		{
			"labels":  nodeLabels,
			"targets": targets,
		},
	}

	return json.Marshal(config)
}

func writeCollectorConfig(metricsFilePath string, config []byte) error {
	file, err := os.OpenFile(metricsFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := file.Write(config); err != nil {
		return err
	}

	return nil
}
