// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"runtime"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/x/contracts/test"
)

// Benchmarks the time it takes to get a module when it is not cached
// BenchmarkRuntimeModuleNotCached-10    	     145	  10262426 ns/op	  245882 B/op	       9 allocs/op
func BenchmarkRuntimeModuleNotCached(b *testing.B) {
	require := require.New(b)

	ctx := context.Background()
	rt := newTestRuntime(ctx)

	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	callInfo := &CallInfo{
		Contract:     contract.Address,
		State:        rt.StateManager,
		FunctionName: "get_value",
		Params:       test.SerializeParams(),
	}
	newInfo, err := rt.callContext.createCallInfo(callInfo)
	require.NoError(err)
	programID, err := callInfo.State.GetAccountContract(ctx, newInfo.Contract)
	require.NoError(err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = rt.callContext.r.getModule(ctx, newInfo, programID)
		require.NoError(err)

		b.StopTimer()
		// reset module
		rt.callContext.r.contractCache.Flush()
		b.StartTimer()
	}
}

// Benchmarks the time it takes to get a module when it is cached
// BenchmarkRuntimeModuleCached-10    	 2429497	       495.1 ns/op	      32 B/op	       1 allocs/op
func BenchmarkRuntimeModuleCached(b *testing.B) {
	require := require.New(b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	b.ResetTimer()

	callInfo := &CallInfo{
		Contract:     contract.Address,
		State:        rt.StateManager,
		FunctionName: "get_value",
		Params:       test.SerializeParams(),
	}
	newInfo, err := rt.callContext.createCallInfo(callInfo)
	require.NoError(err)
	programID, err := callInfo.State.GetAccountContract(ctx, newInfo.Contract)
	require.NoError(err)

	for i := 0; i < b.N; i++ {
		_, err = rt.callContext.r.getModule(ctx, newInfo, programID)
		require.NoError(err)
	}
}

// Benchmarks the time it takes to get an instance which happens on every contract call
// Wasmitime rust allows you to pre-instantiate the module, which is not implemented in go
//
//	https://docs.rs/wasmtime/latest/wasmtime/struct.Linker.html#method.instantiate_pre
//
// BenchmarkRuntimeInstance-10    	   48392	     28894 ns/op	     265 B/op	      15 allocs/op
func BenchmarkRuntimeInstance(b *testing.B) {
	require := require.New(b)

	ctx := context.Background()
	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	b.ResetTimer()

	callInfo := &CallInfo{
		Contract:     contract.Address,
		State:        rt.StateManager,
		FunctionName: "get_value",
		Params:       test.SerializeParams(),
	}
	newInfo, err := rt.callContext.createCallInfo(callInfo)
	require.NoError(err)
	programID, err := newInfo.State.GetAccountContract(ctx, newInfo.Contract)
	require.NoError(err)

	module, err := rt.callContext.r.getModule(ctx, newInfo, programID)
	require.NoError(err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		runtime.GC()
		b.StartTimer()

		inst, err := rt.callContext.r.getInstance(module)
		require.NoError(err)
		_ = inst
	}
}

// Benchmarks the time it takes to call a contract with everything instantiated
// BenchmarkRuntimeInstanceCall-10    	   32728	     33143 ns/op	    1629 B/op	     191 allocs/op
func BenchmarkRuntimeInstanceCall(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	b.ResetTimer()

	callInfo := &CallInfo{
		Contract:     contract.Address,
		State:        rt.StateManager,
		FunctionName: "get_value",
		Params:       test.SerializeParams(),
	}
	newInfo, err := rt.callContext.createCallInfo(callInfo)
	require.NoError(err)
	programID, err := newInfo.State.GetAccountContract(ctx, newInfo.Contract)
	require.NoError(err)

	module, err := rt.callContext.r.getModule(ctx, newInfo, programID)
	require.NoError(err)
	inst, err := rt.callContext.r.getInstance(module)
	require.NoError(err)
	b.ResetTimer()
	newInfo.inst = inst
	rt.callContext.r.setCallInfo(inst.store, newInfo)

	for i := 0; i < b.N; i++ {
		result, err := inst.call(ctx, newInfo)
		require.NoError(err)
		_ = result

		rt.callContext.r.deleteCallInfo(inst.store)
		b.StopTimer()
		// reset module
		module, err = rt.callContext.r.getModule(ctx, newInfo, programID)
		require.NoError(err)
		inst, err = rt.callContext.r.getInstance(module)
		require.NoError(err)
		newInfo.inst = inst
		b.StartTimer()
		rt.callContext.r.setCallInfo(inst.store, newInfo)
	}
}

func BenchmarkRuntimeCallContractBasic(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := contract.CallWithSerializedParams("get_value", nil)
		require.NoError(err)
		_ = result

		b.StopTimer()
		require.Equal(uint64(0), into[uint64](result))
		runtime.GC()
		b.StartTimer()
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

	params := test.SerializeParams(actor)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := contract.CallWithSerializedParams("send_balance", params)
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

	params := test.SerializeParams(counterAddress, addressOf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := contract.CallWithSerializedParams("get_value", params)
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

// Benchmark an NFT
func BenchmarkNFTMint(b *testing.B) {
	require := require.New(b)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	nft, err := rt.newTestContract("nft")
	require.NoError(err)

	_, err = nft.Call("init", "NFT", "NFT")
	require.NoError(err)

	actor := codec.CreateAddress(0, ids.GenerateTestID())
	params := make([][]byte, b.N)

	for i := 0; i < b.N; i++ {
		params[i] = test.SerializeParams(actor, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = nft.CallWithSerializedParams("mint", params[i])
		require.NoError(err)
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

	params := test.SerializeParams(tokenX.Address, 150)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		received, err := amm.WithActor(swaper).CallWithSerializedParams("swap", params)
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
