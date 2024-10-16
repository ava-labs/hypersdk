// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/ava-labs/hypersdk/tests/integration"

	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	morpheusWorkload "github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	testVM := fixture.NewTestVM(0)
	workload.InitSimpleTx(testVM.GetKeys())
	workloadFactory := morpheusWorkload.NewTxGenerator(100)
	genesisBytes, err := testVM.GetGenesisBytes()
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	// Setup imports the integration test coverage
	integration.Setup(
		vm.New,
		genesisBytes,
		lconsts.ID,
		vm.CreateParser,
		workloadFactory,
		randomEd25519AuthFactory,
	)
})
