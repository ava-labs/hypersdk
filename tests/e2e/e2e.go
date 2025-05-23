// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/load"
	"github.com/ava-labs/hypersdk/pubsub"
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

	loadFactory   chain.AuthFactory
	loadIssuers   LoadIssuers
	burstConfig   load.BurstOrchestratorConfig
	gradualConfig load.GradualOrchestratorConfig

	createTransferF CreateTransfer

	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrTxFailed          = errors.New("transaction failed")
)

// LoadIssuers returns the components necessary to instantiate an
// Orchestrator.
// LoadIssuers is an initializer function since the node URIs are not known until runtime.
type LoadIssuers func(
	ctx context.Context,
	uri string,
	authFactories []chain.AuthFactory,
	clients []*ws.WebSocketClient,
	tracker load.Tracker[ids.ID],
) ([]load.Issuer[*chain.Transaction], error)

// CreateTransfer should return an action that transfers amount to the given address
type CreateTransfer func(to codec.Address, amount uint64, nonce uint64) chain.Action

func SetWorkload(
	networkConfigImpl workload.TestNetworkConfiguration,
	workloadTxGenerator workload.TxGenerator,
	abi abi.ABI,
	loadAccount chain.AuthFactory,
	generator LoadIssuers,
	burstConf load.BurstOrchestratorConfig,
	gradualConf load.GradualOrchestratorConfig,
	createTransfer CreateTransfer,
) {
	networkConfig = networkConfigImpl
	txWorkload = workload.TxWorkload{Generator: workloadTxGenerator}
	expectedABI = abi
	loadFactory = loadAccount
	loadIssuers = generator
	burstConfig = burstConf
	gradualConfig = gradualConf
	createTransferF = createTransfer
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

var _ = ginkgo.Describe("[HyperSDK Load Workloads]", ginkgo.Ordered, ginkgo.Serial, func() {
	tc := e2e.NewTestContext()
	r := require.New(tc)

	registry := prometheus.NewRegistry()
	tracker, err := load.NewPrometheusTracker[ids.ID](registry)
	r.NoError(err)

	ginkgo.BeforeAll(func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		ctx := context.Background()

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
			e2e.GetEnv(tc).GetNetwork().UUID,
		)
		require.NoError(err)

		homedir, err := os.UserHomeDir()
		require.NoError(err)

		filePath := filepath.Join(homedir, metricsFilePath)
		require.NoError(writeCollectorConfig(filePath, collectorConfigBytes))

		ginkgo.DeferCleanup(func() {
			require.NoError(metricsServer.Shutdown(ctx))
			require.ErrorIs(metricsServerErr, http.ErrServerClosed)
			require.NoError(os.Remove(filePath))
		})
	})

	ginkgo.It("Short Burst Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		ctx := tc.DefaultContext()
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		uris := getE2EURIs(tc, blockchainID)

		accounts, err := distributeFunds(ctx, tc, createTransferF, loadFactory, uint64(len(uris)))
		require.NoError(err)

		clients := make([]*ws.WebSocketClient, len(uris))
		for i := range clients {
			clients[i], err = ws.NewWebSocketClient(
				uris[i%len(uris)],
				ws.DefaultHandshakeTimeout,
				pubsub.MaxPendingMessages,
				pubsub.MaxReadMessageSize,
			)
			require.NoError(err, "creating websocket client")
		}

		issuers, err := loadIssuers(
			ctx,
			uris[0],
			accounts,
			clients,
			tracker,
		)
		require.NoError(err)

		agents := make([]load.Agent[*chain.Transaction], len(uris))
		for i := range agents {
			listener := load.NewDefaultListener(clients[i], tracker)
			agents[i] = load.NewAgent(issuers[i], listener)
		}

		orchestrator, err := load.NewBurstOrchestrator(
			agents,
			tc.Log(),
			burstConfig,
		)
		require.NoError(err)

		require.NoError(orchestrator.Execute(ctx))

		numTxs := burstConfig.TxsPerIssuer * uint64(len(agents))
		require.Equal(numTxs, tracker.GetObservedIssued())
		require.Equal(numTxs, tracker.GetObservedConfirmed())
		require.Equal(uint64(0), tracker.GetObservedFailed())

		require.NoError(consolidateFunds(ctx, tc, createTransferF, accounts, loadFactory))
	})

	ginkgo.It("Gradual Load Workload", func() {
		tc := e2e.NewTestContext()
		require := require.New(tc)
		ctx := tc.ContextWithTimeout(15 * time.Minute)
		blockchainID := e2e.GetEnv(tc).GetNetwork().GetSubnet(networkConfig.Name()).Chains[0].ChainID
		uris := getE2EURIs(tc, blockchainID)

		accounts, err := distributeFunds(ctx, tc, createTransferF, loadFactory, uint64(len(uris)))
		require.NoError(err)

		clients := make([]*ws.WebSocketClient, len(uris))
		for i := range clients {
			clients[i], err = ws.NewWebSocketClient(
				uris[i%len(uris)],
				ws.DefaultHandshakeTimeout,
				pubsub.MaxPendingMessages,
				pubsub.MaxReadMessageSize,
			)
			require.NoError(err, "creating websocket client")
		}

		issuers, err := loadIssuers(
			ctx,
			uris[0],
			accounts,
			clients,
			tracker,
		)
		require.NoError(err)

		agents := make([]load.Agent[*chain.Transaction], len(uris))
		for i := range agents {
			listener := load.NewDefaultListener(clients[i], tracker)
			agents[i] = load.NewAgent(issuers[i], listener)
		}

		orchestrator, err := load.NewGradualOrchestrator(
			agents,
			tracker,
			tc.Log(),
			gradualConfig,
		)
		require.NoError(err)

		require.NoError(orchestrator.Execute(ctx))

		require.GreaterOrEqual(tracker.GetObservedIssued(), gradualConfig.MaxTPS)

		require.NoError(consolidateFunds(ctx, tc, createTransferF, accounts, loadFactory))
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

func distributeFunds(
	ctx context.Context,
	tc *e2e.GinkgoTestContext,
	createTransfer CreateTransfer,
	funder chain.AuthFactory,
	numAccounts uint64,
) ([]chain.AuthFactory, error) {
	network := NewNetwork(tc)
	ruleFactory, err := network.GetRuleFactory(ctx)
	if err != nil {
		return nil, err
	}

	uris := network.URIs()
	cli := jsonrpc.NewJSONRPCClient(uris[0])
	ws, err := ws.NewWebSocketClient(
		uris[0],
		ws.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		pubsub.MaxReadMessageSize,
	)
	if err != nil {
		return nil, err
	}
	defer ws.Close()

	pacer := newPacer(ws, 100)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go pacer.run(cctx)

	balance, err := cli.GetBalance(ctx, funder.Address())
	if err != nil {
		return nil, err
	}

	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return nil, err
	}

	nonce := uint64(0)

	action := []chain.Action{createTransfer(funder.Address(), 1, nonce)}
	units, err := chain.EstimateUnits(
		ruleFactory.GetRules(time.Now().UnixMilli()),
		action,
		funder,
	)
	if err != nil {
		return nil, err
	}

	fee, err := fees.MulSum(units, unitPrices)
	if err != nil {
		return nil, err
	}

	amountPerAccount := (balance - (numAccounts * fee)) / numAccounts
	if amountPerAccount == 0 {
		return nil, ErrInsufficientFunds
	}

	// Generate test accounts
	accounts := make([]chain.AuthFactory, numAccounts)
	for i := range numAccounts {
		pk, err := ed25519.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		accounts[i] = auth.NewED25519Factory(pk)
	}

	// Send and confirm funds to each account
	for _, account := range accounts {
		action := createTransfer(account.Address(), amountPerAccount, nonce)
		nonce++
		tx, err := chain.GenerateTransaction(
			ruleFactory,
			unitPrices,
			time.Now().UnixMilli(),
			[]chain.Action{action},
			funder,
		)
		if err != nil {
			return nil, err
		}

		if err := pacer.add(tx); err != nil {
			return nil, err
		}
	}

	if err := pacer.wait(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func consolidateFunds(
	ctx context.Context,
	tc *e2e.GinkgoTestContext,
	createTransfer CreateTransfer,
	accounts []chain.AuthFactory,
	to chain.AuthFactory,
) error {
	network := NewNetwork(tc)
	ruleFactory, err := network.GetRuleFactory(ctx)
	if err != nil {
		return err
	}

	uris := network.URIs()
	cli := jsonrpc.NewJSONRPCClient(uris[0])
	ws, err := ws.NewWebSocketClient(
		uris[0],
		ws.DefaultHandshakeTimeout,
		pubsub.MaxPendingMessages,
		pubsub.MaxReadMessageSize,
	)
	if err != nil {
		return err
	}
	defer ws.Close()

	pacer := newPacer(ws, 100)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go pacer.run(cctx)

	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return err
	}

	nonce := uint64(0)

	action := []chain.Action{createTransfer(to.Address(), 1, nonce)}
	units, err := chain.EstimateUnits(
		ruleFactory.GetRules(time.Now().UnixMilli()),
		action,
		to,
	)
	if err != nil {
		return err
	}

	fee, err := fees.MulSum(units, unitPrices)
	if err != nil {
		return err
	}

	balances := make([]uint64, len(accounts))
	for i, account := range accounts {
		balance, err := cli.GetBalance(ctx, account.Address())
		if err != nil {
			return err
		}
		balances[i] = balance
	}

	txs := make([]*chain.Transaction, 0)
	for i, balance := range balances {
		if balance < fee {
			continue
		}
		amount := balance - fee
		action := createTransfer(to.Address(), amount, nonce)
		nonce++
		tx, err := chain.GenerateTransaction(
			ruleFactory,
			unitPrices,
			time.Now().UnixMilli(),
			[]chain.Action{action},
			accounts[i],
		)
		if err != nil {
			return err
		}
		txs = append(txs, tx)
	}

	for _, tx := range txs {
		if err := pacer.add(tx); err != nil {
			return err
		}
	}

	return pacer.wait()
}

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
