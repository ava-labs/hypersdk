// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
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
		Contract:      contract.Address,
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
		Contract:      contract.Address,
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
//  https://docs.rs/wasmtime/latest/wasmtime/struct.Linker.html#method.instantiate_pre
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
		Contract:      contract.Address,
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
		inst, err := rt.callContext.r.getInstance(module, rt.callContext.r.hostImports)
		require.NoError(err)
		_ = inst

		b.StopTimer()
		// reset module
		module, err = rt.callContext.r.getModule(ctx, newInfo, programID)
		require.NoError(err)
		b.StartTimer()
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
		Contract:      contract.Address,
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
	inst, err := rt.callContext.r.getInstance(module, rt.callContext.r.hostImports)
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
		inst, err = rt.callContext.r.getInstance(module, rt.callContext.r.hostImports)
		require.NoError(err)
		newInfo.inst = inst
		b.StartTimer()
		rt.callContext.r.setCallInfo(inst.store, newInfo)
	}
}

func BenchmarkRuntimeCallContractBasic(b *testing.B) {
	require := require.New(b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	for i := 0; i < b.N; i++ {
		result, err := contract.Call("get_value")
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

func TestRuntimeCallContractBasicAttachValue(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("return_complex_type")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(ComplexReturn{Contract: contract.Address, MaxUnits: 1000}, into[ComplexReturn](result))
}
