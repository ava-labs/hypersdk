// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tests/registry"
	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestsRegistry initialized during init to ensure tests are identical during ginkgo
// suite construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

var _ = registry.Register(TestsRegistry, "EVM Calls", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	networkConfig := tn.Configuration().(*workload.NetworkConfiguration)
	spendingKey := networkConfig.Keys()[0]

	cli := jsonrpc.NewJSONRPCClient(tn.URIs()[0])

	toAddressEVM := storage.ConvertAddress(toAddress)

	call := &actions.EvmCall{
		To:       &toAddressEVM,
		Value:    big.NewInt(1),
		GasLimit: 1000000,
		Data:     nil,
		Keys:     state.Keys{}, // will be populated by simulator
	}
	// panics
	simRes, err := cli.SimulateActions(context.Background(), chain.Actions{call}, auth.NewED25519Address(networkConfig.Keys()[0].PublicKey()))
	require.NoError(err)
	fmt.Println("simRes", simRes)

	// if simRes[0].StateKeys != nil {
	// 	call.Keys = simRes[0].StateKeys
	// }

	tx, err := tn.GenerateTx(context.Background(), []chain.Action{call},
		auth.NewED25519Factory(spendingKey),
	)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
})
