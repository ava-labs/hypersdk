// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/tests/integration"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/onsi/ginkgo/v2"

	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	morpheusWorkload "github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())
	gen, workloadFactory, err := morpheusWorkload.New(0 /* minBlockGap: 0ms */)
	require.NoError(err)

	parser := controller.NewParser(0, ids.Empty, gen)
	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	vmF := func(options ...vm.Option) *vm.VM {
		return controller.New(logging.NoLog{}, trace.Noop, options...)
	}

	// Setup imports the integration test coverage
	integration.Setup(
		vmF,
		genesisBytes,
		lconsts.ID,
		parser,
		controller.JSONRPCEndpoint,
		workloadFactory,
		randomEd25519AuthFactory,
	)
})
