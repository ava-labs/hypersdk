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
		targetFactory := authFactories[0]
		authFactory := tn.Configuration().AuthFactories()[0]

		client := jsonrpc.NewJSONRPCClient(tn.URIs()[0])
		balance, err := client.GetBalance(ctx, targetFactory.Address())
		require.NoError(err)
		require.Equal(uint64(1000), balance)

		tx, err := tn.GenerateTx(ctx, []chain.Action{&actions.Transfer{
			To:    targetFactory.Address(),
			Value: 1,
		}},
			authFactory,
		)
		require.NoError(err)

		timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
		defer timeoutCtxFnc()
		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

		balance, err = client.GetBalance(ctx, targetFactory.Address())
		require.NoError(err)
		require.Equal(uint64(1001), balance)
	}, 1000)
