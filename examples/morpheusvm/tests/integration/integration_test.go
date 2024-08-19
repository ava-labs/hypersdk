// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/tests/integration"

	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	morpheusWorkload "github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

func TestIntegration(t *testing.T) {
	ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())
	gen, workloadFactory, err := morpheusWorkload.New(0 /* minBlockGap: 0ms */)
	require.NoError(err)

	parser := lrpc.NewParser(0, ids.Empty, gen)
	genesisBytes, err := json.Marshal(gen)
	require.NoError(err)

	randomEd25519Priv, err := ed25519.GeneratePrivateKey()
	require.NoError(err)

	randomEd25519AuthFactory := auth.NewED25519Factory(randomEd25519Priv)

	// Setup imports the integration test coverage
	integration.Setup(
		controller.New,
		genesisBytes,
		lconsts.ID,
		parser,
		rpc.JSONRPCEndpoint,
		workloadFactory,
		randomEd25519AuthFactory,
	)
})

// ginkgo.It("responds on ABI request with a valid json response", func() {
// 		abi, err := instances[0].lcli.GetABI(context.TODO())
// 		require.NoError(err)

// 		var abiJSON []map[string]interface{}
// 		err = json.Unmarshal([]byte(abi), &abiJSON)
// 		require.NoError(err)

// 		obj := abiJSON[0]
// 		require.Equal(obj["id"], float64(0)) // JSON numbers are parsed as float64
// 		require.Equal(obj["name"], "Transfer")
// 	})
