// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/examples/testvm/consts"
	"github.com/ava-labs/hypersdk/examples/testvm/tests/workload"
	"github.com/ava-labs/hypersdk/examples/testvm/vm"
	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/stretchr/testify/require"

	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "testvm-e2e-tests"

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "testvm e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single testvm Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	keys := workload.NewDefaultKeys()
	genesis := workload.NewGenesis(keys, 100*time.Millisecond)
	genesisBytes, err := json.Marshal(genesis)
	require.NoError(err)
	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	parser, err := vm.CreateParser(genesisBytes)
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject testvm name
	// and workload factory to orchestrate the test.
	generator := workload.NewTxGenerator(keys[0])
	tc := e2e.NewTestContext()
	he2e.SetWorkload(consts.Name, generator, expectedABI, parser, nil, nil)

	return fixture.NewTestEnvironment(tc, flagVars, owner, consts.Name, consts.ID, genesisBytes).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})
