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

// create a single tests registry instance. This doesn't need to be duplicated if we were to add
// additional tests.
var TestsRegistry = &registry.Registry{}

var _ = registry.Register(TestsRegistry, "Transfer Transaction", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) error {
	require := require.New(t)

	firstNode := tn.Nodes()[0]
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	networkConfig := tn.Configuration().(*workload.NetworkConfiguration)
	spendingKey := networkConfig.Keys()[0]

	tx, err := firstNode.GenerateTx(context.Background(), []chain.Action{&actions.Transfer{
		To:    toAddress,
		Value: 1,
	}},
		auth.NewED25519Factory(spendingKey),
	)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer timeoutCtxFnc()

	require.NoError(firstNode.SubmitTxs(timeoutCtx, []*chain.Transaction{tx}))
	require.NoError(firstNode.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
	return nil
})
