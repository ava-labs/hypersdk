// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/tests/registry"

	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestsRegistry initialized during init to ensure tests are identical during ginkgo
// suite construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

var _ = registry.Register(TestsRegistry, "Transfer Transaction",
	func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork, authFactories []chain.AuthFactory) {
		require := require.New(t)
		ctx := context.Background()
		sourceAuthFactory, targetAuthFactory := authFactories[0], authFactories[1]

		client := jsonrpc.NewJSONRPCClient(tn.URIs()[0])
		sourceBalance, err := client.GetBalance(ctx, sourceAuthFactory.Address())
		require.NoError(err)
		require.Equal(uint64(1000000), sourceBalance)
		targetBalance, err := client.GetBalance(ctx, targetAuthFactory.Address())
		require.NoError(err)
		require.Equal(uint64(1000000), targetBalance)

		tx, err := tn.GenerateTx(ctx, []chain.Action{&actions.Transfer{
			To:    targetAuthFactory.Address(),
			Value: 1,
		}},
			sourceAuthFactory,
		)
		require.NoError(err)

		timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
		defer timeoutCtxFnc()
		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

		sourceBalance, err = client.GetBalance(ctx, sourceAuthFactory.Address())
		require.NoError(err)
		require.True(uint64(1000000) > sourceBalance)
		targetBalance, err = client.GetBalance(ctx, targetAuthFactory.Address())
		require.NoError(err)
		require.Equal(uint64(1000001), targetBalance)
	}, 1000000, 1000000)
