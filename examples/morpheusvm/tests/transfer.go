// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/tests/registry"

	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// Global test registry initialized during init to ensure tests are identical during ginkgo
// tree construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

var _ = registry.Register(TestsRegistry, "Transfer Transaction", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	networkConfig := tn.Configuration().(*workload.NetworkConfiguration)
	spendingKey := networkConfig.Keys()[0]

	tx, err := tn.GenerateTx(context.Background(), []chain.Action{&actions.Transfer{
		To:    toAddress,
		Value: 1,
	}},
		auth.NewED25519Factory(spendingKey),
	)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
})
