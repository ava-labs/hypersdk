// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	_ "github.com/ava-labs/hypersdk/examples/morpheusvm/tests" // include the tests shared between integration and e2e

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
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
		authFactories[2],
		loadTxGenerators,
		hload.ShortBurstOrchestratorConfig{
			TxsPerIssuer: 1_000,
			Timeout:      20 * time.Second,
		},
		hload.DefaultGradualLoadOrchestratorConfig(),
		createTransfer,
	)

	tc := e2e.NewEventHandlerTestContext()
	return fixture.NewTestEnvironment(tc, flagVars, owner, testingNetworkConfig, consts.ID).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(e2e.NewEventHandlerTestContext(), envBytes)
})

func createTransfer(to codec.Address, amount uint64, nonce uint64) chain.Action {
	return &actions.Transfer{
		To:    to,
		Value: amount,
		Memo:  binary.BigEndian.AppendUint64(nil, nonce),
	}
}

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
