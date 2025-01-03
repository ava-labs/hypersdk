// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/tests/registry"

	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestsRegistry initialized during init to ensure tests are identical during ginkgo
// suite construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

var _ = registry.Register(TestsRegistry, "Transfer Transaction", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	authFactory := tn.Configuration().AuthFactories()[0]
	tx, err := tn.GenerateTx(context.Background(), []chain.Action{&actions.Transfer{
		To:    toAddress,
		Value: 1,
	}},
		authFactory,
	)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
})

var _ = registry.Register(TestsRegistry, "Read From Memory Refund", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	// We first warm up the state we want to test
	authFactory := tn.Configuration().AuthFactories()[0]
	tx, err := tn.GenerateTx(context.Background(), []chain.Action{&actions.Transfer{
		To:    toAddress,
		Value: 1,
		Memo:  []byte("warming up keys"),
	}},
		authFactory,
	)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	// This call implicitly enforces that the state is warmed up by having the
	// VM produce + accept a new block
	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	indexerCli := indexer.NewClient(tn.URIs()[0])
	resp, success, err := indexerCli.GetTx(timeoutCtx, tx.GetID())
	require.True(success)
	require.NoError(err)
	coldFee := resp.Fee
	coldUnits := resp.Units

	// If TX successful, state is now warmed up
	// Now we can test the refund
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{&actions.Transfer{
		To:    toAddress,
		Value: 1,
		Memo:  []byte("testing refunds"),
	}},
		authFactory,
	)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	queryCtx, queryCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer queryCtxFnc()

	resp, success, err = indexerCli.GetTx(queryCtx, tx.GetID())
	require.NoError(err)
	require.True(success)

	require.Less(resp.Fee, coldFee)
	require.Less(resp.Units[fees.StorageRead], coldUnits[fees.StorageRead])
})
