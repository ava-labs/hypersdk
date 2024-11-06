// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/tests/integration"
	"github.com/ava-labs/hypersdk/x/contracts/vm/vm"

	lconsts "github.com/ava-labs/hypersdk/x/contracts/vm/consts"
	vmwithcontractsWorkload "github.com/ava-labs/hypersdk/x/contracts/vm/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "vmwithcontracts integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())
	genesis, workloadFactory, err := vmwithcontractsWorkload.New(0 /* minBlockGap: 0ms */)
	require.NoError(err)

	genesisBytes, err := json.Marshal(genesis)
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
