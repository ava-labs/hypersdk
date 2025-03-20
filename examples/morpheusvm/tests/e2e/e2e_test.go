// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
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

	testingNetworkConfig, err := workload.NewTestNetworkConfig(100 * time.Millisecond)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	authFactories := testingNetworkConfig.AuthFactories()
	generator := workload.NewTxGenerator(authFactories[1])

	he2e.SetWorkload(
		testingNetworkConfig,
		generator,
		expectedABI,
		shortBurstComponentsGenerator,
		hload.ShortBurstOrchestratorConfig{
			N:       1_000,
			Timeout: 20 * time.Second,
		},
	)

	return fixture.NewTestEnvironment(e2e.NewTestContext(), flagVars, owner, testingNetworkConfig, consts.ID).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})

func shortBurstComponentsGenerator(
	ctx context.Context,
	uris []string,
	authFactories []chain.AuthFactory,
) ([]hload.TxGenerator[*chain.Transaction], []hload.Issuer[*chain.Transaction], hload.Tracker[ids.ID], error) {
	lcli := vm.NewJSONRPCClient(uris[0])
	ruleFactory, err := lcli.GetRuleFactory(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	numOfFactories := len(authFactories)
	balances := make([]uint64, numOfFactories)
	// Get balances
	for i, factory := range authFactories {
		balance, err := lcli.Balance(ctx, factory.Address())
		if err != nil {
			return nil, nil, nil, err
		}
		balances[i] = balance
	}

	cli := jsonrpc.NewJSONRPCClient(uris[0])
	unitPrices, err := cli.UnitPrices(ctx, false)
	if err != nil {
		return nil, nil, nil, err
	}

	// Create tx generator
	txGenerators := make([]hload.TxGenerator[*chain.Transaction], numOfFactories)
	for i := 0; i < numOfFactories; i++ {
		txGenerators[i] = load.NewTxGenerator(authFactories[i], ruleFactory, balances[i], unitPrices)
	}

	tracker := &hload.DefaultTracker[ids.ID]{}

	// Use only one issuer if each authFactories can't have its own issuer
	if len(authFactories) != len(uris) {
		issuer, err := hload.NewDefaultIssuer(uris[0], tracker)
		if err != nil {
			return nil, nil, nil, err
		}
		return txGenerators, []hload.Issuer[*chain.Transaction]{issuer}, tracker, nil
	}

	// Create issuers for each authFactory
	issuers := make([]hload.Issuer[*chain.Transaction], len(uris))
	for i := 0; i < len(uris); i++ {
		issuer, err := hload.NewDefaultIssuer(uris[i], tracker)
		if err != nil {
			return nil, nil, nil, err
		}
		issuers[i] = issuer
	}
	return txGenerators, issuers, tracker, nil
}
