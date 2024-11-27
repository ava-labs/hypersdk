// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests"
	_ "github.com/ava-labs/hypersdk/examples/morpheusvm/tests" // include the tests that are shared between the integration and e2e

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/integration"

	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	customAllocs, err := tests.TestsRegistry.GenerateCustomAllocations(auth.GenerateED25519AuthFactory)
	require.NoError(err)
	testingNetworkConfig, err := workload.NewTestNetworkConfig(0, customAllocs)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	generator := workload.NewTxGenerator(testingNetworkConfig.AuthFactories()[0])
	// Setup imports the integration test coverage
	integration.Setup(
		vm.New,
		testingNetworkConfig,
		lconsts.ID,
		generator,
		randomEd25519AuthFactory,
	)
})
