// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/registry"
	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestsRegistry initialized during init to ensure tests are identical during ginkgo
// suite construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

// const txCheckInterval = 100 * time.Millisecond

var _ = registry.Register(TestsRegistry, "EVM Calls", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)
	other, err := ed25519.GeneratePrivateKey()
	require.NoError(err)
	toAddress := auth.NewED25519Address(other.PublicKey())

	networkConfig := tn.Configuration().(*workload.NetworkConfiguration)
	spendingKey := networkConfig.Keys()[0]

	spendingKeyAuthFactory := auth.NewED25519Factory(spendingKey)
	spendingKeyAddr := auth.NewED25519Address(networkConfig.Keys()[0].PublicKey())

	cli := jsonrpc.NewJSONRPCClient(tn.URIs()[0])
	lcli := vm.NewJSONRPCClient(tn.URIs()[0])
	indexerClient := indexer.NewClient(tn.URIs()[0])

	toAddressEVM := storage.ConvertAddress(toAddress)

	transferEVM := &actions.EvmCall{
		To:       &toAddressEVM,
		Value:    1,
		GasLimit: 1000000,
	}

	simRes, err := cli.SimulateActions(context.Background(), chain.Actions{transferEVM}, spendingKeyAddr)
	require.NoError(err)
	require.Len(simRes, 1)
	actionResult := simRes[0]
	evmCallOutputBytes := actionResult.Output
	reader := codec.NewReader(evmCallOutputBytes, len(evmCallOutputBytes))
	evmCallResultTyped, err := vm.OutputParser.Unmarshal(reader)
	require.NoError(err)
	evmCallResult, ok := evmCallResultTyped.(*actions.EvmCallResult)
	require.True(ok)
	evmCallStateKeys := actionResult.StateKeys
	require.True(evmCallResult.Success)
	transferEVM.Keys = evmCallStateKeys

	tx, err := tn.GenerateTx(context.Background(), []chain.Action{transferEVM}, spendingKeyAuthFactory)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	response, included, err := indexerClient.GetTx(timeoutCtx, tx.ID())
	require.NoError(err)
	require.True(included)
	require.True(response.Success)

	fmt.Println("response", response)
	fmt.Println("sender", spendingKeyAddr)
	fmt.Println("senderEVM", storage.ConvertAddress(spendingKeyAddr))
	fmt.Println("toAddr", toAddress)
	fmt.Println("toAddrEVM", toAddressEVM)
	balanceTo, err := lcli.Balance(timeoutCtx, toAddress)
	require.NoError(err)
	require.Equal(balanceTo, uint64(1))
	// are we converting between uint64 and big.Int correctly and handling nil values?
	// are we handling address conversions correctly?
})
