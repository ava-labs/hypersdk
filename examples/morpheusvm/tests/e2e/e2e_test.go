// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"

	_ "github.com/ava-labs/hypersdk/examples/morpheusvm/tests" // include the tests shared between integration and e2e

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/load"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"

	hload "github.com/ava-labs/hypersdk/load"
	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "morpheusvm-e2e-tests"

const (
	metricsURI      = "localhost:8080"
	metricsFilePath = ".tmpnet/prometheus/file_sd_configs/hypersdk-e2e-load-generator-metrics.json"
)

var (
	flagVars *e2e.FlagVars
	registry *prometheus.Registry
)

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single MorpheusVM Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	testingNetworkConfig, err := workload.NewTestNetworkConfig(100 * time.Millisecond)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	authFactories := testingNetworkConfig.AuthFactories()
	generator := workload.NewTxGenerator(authFactories[1])

	reg := prometheus.NewRegistry()
	tracker, err := hload.NewPrometheusTracker[ids.ID](reg)
	require.NoError(err)

	registry = reg

	he2e.SetWorkload(
		testingNetworkConfig,
		generator,
		expectedABI,
		loadTxGenerators,
		tracker,
		hload.ShortBurstOrchestratorConfig{
			TxsPerIssuer: 1_000,
			Timeout:      20 * time.Second,
		},
		hload.DefaultGradualLoadOrchestratorConfig(),
	)

	vmConfig := `{
		"statesync": {"minBlocks": 128},
		"vm": {
			"mempoolSize"       : 9223372036854775807,
			"mempoolSponsorSize": 9223372036854775807
		}
	}`

	return fixture.NewTestEnvironment(e2e.NewTestContext(), flagVars, owner, testingNetworkConfig, consts.ID, vmConfig).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)

	tc := e2e.NewTestContext()
	r := require.New(tc)

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

	go func() {
		r.ErrorIs(
			metricsServer.ListenAndServe(),
			http.ErrServerClosed,
		)
	}()

	// Generate collector config
	collectorConfigBytes, err := generateCollectorConfig(
		[]string{metricsURI},
		e2e.GetEnv(tc).GetNetwork().UUID,
	)
	r.NoError(err)

	r.NoError(writeCollectorConfig(collectorConfigBytes))

	ginkgo.DeferCleanup(func() {
		homeDir, err := os.UserHomeDir()
		r.NoError(err)
		r.NoError(os.Remove(filepath.Join(homeDir, metricsFilePath)))
		r.NoError(metricsServer.Shutdown(context.Background()))
	})
})

func loadTxGenerators(
	ctx context.Context,
	uri string,
	authFactories []chain.AuthFactory,
) ([]hload.TxGenerator[*chain.Transaction], error) {
	lcli := vm.NewJSONRPCClient(uri)
	ruleFactory, err := lcli.GetRuleFactory(ctx)
	if err != nil {
		return nil, err
	}

	numFactories := len(authFactories)
	balances := make([]uint64, numFactories)
	// Get balances
	for i, factory := range authFactories {
		balance, err := lcli.Balance(ctx, factory.Address())
		if err != nil {
			return nil, err
		}
		balances[i] = balance
	}

	cli := jsonrpc.NewJSONRPCClient(uri)
	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return nil, err
	}

	// Create tx generator
	txGenerators := make([]hload.TxGenerator[*chain.Transaction], numFactories)
	for i := 0; i < numFactories; i++ {
		txGenerators[i] = load.NewTxGenerator(authFactories[i], ruleFactory, balances[i], unitPrices)
	}

	return txGenerators, nil
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

func writeCollectorConfig(config []byte) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fmt.Println(homeDir)
	file, err := os.OpenFile(filepath.Join(homeDir, metricsFilePath), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(config); err != nil {
		return err
	}

	return nil
}
