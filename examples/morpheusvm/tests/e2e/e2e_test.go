// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/generator"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/throughput"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
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

	txCheckInterval := 100 * time.Millisecond
	testVM := fixture.NewTestVM(txCheckInterval)
	keys := testVM.GetKeys()
	generator := generator.NewTxGenerator(keys[0], txCheckInterval)
	genesisBytes, err := testVM.GetGenesisBytes()
	require.NoError(err)
	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	parser, err := vm.CreateParser(genesisBytes)
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject MorpheusVM name
	// and workload factory to orchestrate the test.
	spamHelper := throughput.SpamHelper{
		KeyType: auth.ED25519Key,
	}

	tc := e2e.NewTestContext()
	he2e.SetWorkload(consts.Name, generator, expectedABI, parser, &spamHelper, keys[0])

	return fixture.NewTestEnvironment(tc, flagVars, owner, consts.Name, consts.ID, genesisBytes).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})
