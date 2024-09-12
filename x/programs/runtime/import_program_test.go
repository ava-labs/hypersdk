// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestImportProgramDeployProgram(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("deploy_contract")
	require.NoError(err)

	runtime := contract.Runtime
	otherProgramID := ids.GenerateTestID()
	err = runtime.AddProgram(otherProgramID[:], codec.CreateAddress(0, otherProgramID), "call_contract")
	require.NoError(err)

	result, err := contract.Call(
		"deploy",
		otherProgramID[:])
	require.NoError(err)

	newAccount := into[codec.Address](result)

	result, err = runtime.CallProgram(newAccount, "simple_call")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))
}

func TestImportProgramCallProgram(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("call_contract")
	require.NoError(err)

	expected, err := Serialize(0)
	require.NoError(err)

	result, err := contract.Call("simple_call")
	require.NoError(err)
	require.Equal(expected, result)

	result, err = contract.Call(
		"simple_call_external",
		contract.Address, uint64(1000000))
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramActor(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("call_contract")
	require.NoError(err)
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := contract.WithActor(actor).Call("actor_check")
	require.NoError(err)
	expected, err := Serialize(actor)
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramActorChange(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("call_contract")
	require.NoError(err)
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := contract.WithActor(actor).Call(
		"actor_check_external",
		contract.Address, uint64(100000))
	require.NoError(err)
	expected, err := Serialize(contract.Address)
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramWithParam(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("call_contract")
	require.NoError(err)

	expected, err := Serialize(uint64(1))
	require.NoError(err)

	result, err := contract.Call(
		"call_with_param",
		uint64(1))
	require.NoError(err)
	require.Equal(expected, result)

	result, err = contract.Call(
		"call_with_param_external",
		contract.Address, uint64(100000), uint64(1))
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramWithParams(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("call_contract")
	require.NoError(err)

	expected, err := Serialize(int64(3))
	require.NoError(err)

	result, err := contract.Call(
		"call_with_two_params",
		uint64(1),
		uint64(2))
	require.NoError(err)
	require.Equal(expected, result)

	result, err = contract.Call(
		"call_with_two_params_external",
		contract.Address, uint64(100000), uint64(1), uint64(2))
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportGetRemainingFuel(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("fuel")
	require.NoError(err)

	result, err := contract.Call("get_fuel")
	require.NoError(err)
	require.LessOrEqual(into[uint64](result), contract.Runtime.callContext.defaultCallInfo.Fuel)
}

func TestImportOutOfFuel(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestProgram("fuel")
	require.NoError(err)

	result, err := contract.Call("out_of_fuel", contract.Address)
	require.NoError(err)
	require.Equal([]byte{byte(OutOfFuel)}, result)
}
