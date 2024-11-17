// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

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

	cli := jsonrpc.NewJSONRPCClient(tn.URIs()[1])
	lcli := vm.NewJSONRPCClient(tn.URIs()[1])
	// indexerCli := indexer.NewClient(tn.URIs()[0])

	toAddressEVM := storage.ConvertAddress(toAddress)

	transferEVM := &actions.EvmCall{
		To:       &toAddressEVM,
		Value:    common.Big1,
		GasLimit: 1000000,
	}

	// balanceSender, err := cli.GetBalance(context.Background(), auth.NewED25519Address(networkConfig.Keys()[0].PublicKey()))
	// require.NoError(err)
	// require.Equal(balanceSender, uint64(9999999690990))
	simRes, err := cli.SimulateActions(context.Background(), chain.Actions{transferEVM}, auth.NewED25519Address(networkConfig.Keys()[0].PublicKey()))
	require.NoError(err)
	fmt.Println("simRes", simRes[0].StateKeys)
	out := simRes[0].Output
	reader := codec.NewReader(out, len(out))
	callOutputTyped, err := vm.OutputParser.Unmarshal(reader)
	require.NoError(err)
	callOutput, ok := callOutputTyped.(*actions.EvmCallResult)
	require.True(ok)
	fmt.Println("simRes Success:", callOutput.Success)
	if !callOutput.Success {
		fmt.Println("simRes Error:", callOutput.Err)
	}
	fmt.Println("simRes Return Data:", callOutput.Return)

	if simRes[0].StateKeys != nil {
		transferEVM.Keys = simRes[0].StateKeys
	} else {
		require.Fail(fmt.Sprintf("no keys: %v", simRes[0]))
	}

	// time.Sleep(11 * time.Millisecond)
	tx, err := tn.GenerateTx(context.Background(), []chain.Action{transferEVM},
		auth.NewED25519Factory(spendingKey),
	)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
	balanceTo, err := lcli.Balance(timeoutCtx, toAddress)
	fmt.Println("balanceTo", balanceTo)
	require.NoError(err)
	require.Equal(balanceTo, uint64(1))

	// success, _, err := indexerCli.WaitForTransaction(timeoutCtx, txCheckInterval, tx.ID())
	// require.NoError(err)
	// require.True(success)

	// callRes, _, err := indexerCli.GetTx(timeoutCtx, tx.ID())
	// require.NoError(err)
	// callOutputBytes := []byte(callRes.Outputs[0])
	// require.Equal(consts.EvmCallID, callOutputBytes[0])
	// reader := codec.NewReader(callOutputBytes, len(callOutputBytes))
	// callOutputTyped, err := vm.OutputParser.Unmarshal(reader)
	// require.NoError(err)
	// callOutput, ok := callOutputTyped.(*actions.EvmCallResult)
	// require.True(ok)
	// require.True(callOutput.Success)
})
