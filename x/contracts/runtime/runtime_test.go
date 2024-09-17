// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// Benchmarks calling a contract that returns 0 immediately
func BenchmarkRuntimeCallContractBasic(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := contract.Call("get_value")
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

// Benchmarks a contract that sends native balance.
func BenchmarkRuntimeSendValue(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	actor := codec.CreateAddress(0, ids.GenerateTestID())

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("balance")
	require.NoError(err)
	contract.Runtime.StateManager.(TestStateManager).Balances[contract.Address] = consts.MaxUint64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := contract.Call("send_balance", actor)
		require.NoError(err)
		require.True(into[bool](result))
	}
}

func BenchmarkRuntimeBasicExternalCalls(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("counter-external")
	require.NoError(err)

	contractID := ids.GenerateTestID()
	stringedID := string(contractID[:])
	counterAddress := codec.CreateAddress(0, contractID)
	err = rt.AddContract(ContractID(stringedID), counterAddress, "counter")
	require.NoError(err)
	addressOf := codec.CreateAddress(0, ids.GenerateTestID())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := contract.Call("get_value", counterAddress, addressOf)
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

// Benchmarks a contract that performs an AMM swap
func BenchmarkAmmSwaps(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	tokenX, err := rt.newTestContract("token")
	require.NoError(err)
	tokenY, err := rt.newTestContract("token")
	require.NoError(err)

	amountMint := uint64((1 << 28))

	// initialize the tokens
	_, err = tokenX.Call("init", "tokenX", "TKX")
	require.NoError(err)
	_, err = tokenY.Call("init", "tokenY", "TKY")
	require.NoError(err)

	swaper := codec.CreateAddress(0, ids.GenerateTestID())
	lp := codec.CreateAddress(0, ids.GenerateTestID())

	// mint tokens for actors
	_, err = tokenX.Call("mint", swaper, amountMint)
	require.NoError(err)
	_, err = tokenX.Call("mint", lp, amountMint)
	require.NoError(err)
	_, err = tokenY.Call("mint", lp, amountMint)
	require.NoError(err)

	// create and initialize the amm
	amm, err := rt.newTestContract("automated-market-maker")
	require.NoError(err)
	_, err = amm.Call("init", tokenX.Address, tokenY.Address, tokenX.ID)
	require.NoError(err)

	// approve tokens
	_, err = tokenX.WithActor(lp).Call("approve", amm.Address, amountMint)
	require.NoError(err)
	_, err = tokenY.WithActor(lp).Call("approve", amm.Address, amountMint)
	require.NoError(err)
	_, err = tokenX.WithActor(swaper).Call("approve", amm.Address, amountMint)
	require.NoError(err)

	// add liquidity
	_, err = amm.WithActor(lp).Call("add_liquidity", amountMint, amountMint)
	require.NoError(err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		received, err := amm.WithActor(swaper).Call("swap", tokenX.Address, 150)
		require.NoError(err)
		require.NotZero(received)
	}
}

func TestRuntimeCallContractBasicAttachValue(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	actor := codec.CreateAddress(0, ids.GenerateTestID())
	contract.Runtime.StateManager.(TestStateManager).Balances[actor] = 10

	actorBalance, err := contract.Runtime.StateManager.GetBalance(context.Background(), actor)
	require.NoError(err)
	require.Equal(uint64(10), actorBalance)

	// calling a contract with a value transfers that amount from the caller to the contract
	result, err := contract.WithActor(actor).WithValue(4).Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	actorBalance, err = contract.Runtime.StateManager.GetBalance(context.Background(), actor)
	require.NoError(err)
	require.Equal(uint64(6), actorBalance)

	contractBalance, err := contract.Runtime.StateManager.GetBalance(context.Background(), contract.Address)
	require.NoError(err)
	require.Equal(uint64(4), contractBalance)
}

func TestRuntimeCallContractBasic(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))
}

type ComplexReturn struct {
	Contract codec.Address
	MaxUnits uint64
}

func TestRuntimeCallContractComplexReturn(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("return_complex_type")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(ComplexReturn{Contract: contract.Address, MaxUnits: 1000}, into[ComplexReturn](result))
}
