// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/tests/fixture"

	he2e "github.com/ava-labs/hypersdk/tests/e2e"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const owner = "morpheusvm-e2e-tests"

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {
	flagVars := e2e.RegisterFlags()
	require := require.New(ginkgo.GinkgoT())

	gen, workloadFactory, err := workload.New(100 /* minBlockGap: 100ms */)
	require.NoError(err)

	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	he2e.SetWorkload(consts.Name, workloadFactory)

	// Run only once in the first ginkgo process
	nodes := tmpnet.NewNodesOrPanic(flagVars.NodeCount())
	subnet := fixture.NewHyperVMSubnet(
		consts.Name,
		consts.ID,
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
})
