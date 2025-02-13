// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tests/registry"

	tworkload "github.com/ava-labs/hypersdk/tests/workload"
)

var TestsRegistry = &registry.Registry{}

func TransferTest(t require.TestingT, tn tworkload.TestNetwork) {
	fmt.Println("Running TransferTest")
	r := require.New(t)

	other, err := ed25519.GeneratePrivateKey()
	r.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	authFactory := tn.Configuration().AuthFactories()[0]

	tx, err := tn.GenerateTx(context.Background(), []chain.Action{&actions.EvmCall{
		To:       storage.ToEVMAddress(toAddress),
		Value:    1,
		GasLimit: 21_000,
		Keys:     state.Keys{},
		From:     storage.ToEVMAddress(authFactory.Address()),
	}},
		authFactory,
	)
	r.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	r.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
}

var _ = registry.Register(TestsRegistry, "Transfer", TransferTest)
