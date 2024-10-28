// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/ava-labs/hypersdk/x/contracts/vm/consts"
	"github.com/ava-labs/hypersdk/x/contracts/vm/tests/workload"
	"github.com/ava-labs/hypersdk/x/contracts/vm/vm"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "vmwithcontracts-e2e-tests"

var flagVars *e2e.FlagVars

func TestE2e(t *testing.T) {
	// ginkgo.RunSpecs(t, "vmwithcontracts e2e test suites")
}

func init() {
	flagVars = e2e.RegisterFlags()
}

// Construct tmpnet network with a single VMWithContracts Subnet
var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	require := require.New(ginkgo.GinkgoT())

	gen, workloadFactory, err := workload.New(100 /* minBlockGap: 100ms */)
	require.NoError(err)

	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	expectedABI, err := abi.NewABI(vm.ActionParser.GetRegisteredTypes(), vm.OutputParser.GetRegisteredTypes())
	require.NoError(err)

	parser, err := vm.CreateParser(genesisBytes)
	require.NoError(err)

	// Import HyperSDK e2e test coverage and inject VMWithContracts name
	// and workload factory to orchestrate the test.
	he2e.SetWorkload(consts.Name, workloadFactory, expectedABI, parser, nil, nil)

	tc := e2e.NewTestContext()

	return fixture.NewTestEnvironment(tc, flagVars, owner, consts.Name, consts.ID, genesisBytes).Marshal()
}, func(envBytes []byte) {
	// Run in every ginkgo process

	// Initialize the local test environment from the global state
	e2e.InitSharedTestEnvironment(ginkgo.GinkgoT(), envBytes)
})
