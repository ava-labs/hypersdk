// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestImportProgramCallProgram(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")

	expected, err := serialize(0)
	require.NoError(err)

	result, err := program.Call("simple_call")
	require.NoError(err)
	require.Equal(expected, result)

	result, err = program.Call(
		"simple_call_external",
		program.Address, uint64(1000000))
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramActor(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := program.CallWithActor(actor, "actor_check")
	require.NoError(err)
	expected, err := serialize(actor)
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramActorChange(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := program.CallWithActor(
		actor,
		"actor_check_external",
		program.Address, uint64(100000))
	require.NoError(err)
	expected, err := serialize(program.Address)
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramWithParam(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")

	expected, err := serialize(uint64(1))
	require.NoError(err)

	result, err := program.Call(
		"call_with_param",
		uint64(1))
	require.NoError(err)
	require.Equal(expected, result)

	result, err = program.Call(
		"call_with_param_external",
		program.Address, uint64(100000), uint64(1))
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramWithParams(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")

	expected, err := serialize(int64(3))
	require.NoError(err)
	result, err := program.Call(
		"call_with_two_params",
		uint64(1),
		uint64(2))
	require.NoError(err)
	require.Equal(expected, result)

	result, err = program.Call(
		"call_with_two_params_external",
		program.Address, uint64(100000), uint64(1), uint64(2))
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportGetRemainingFuel(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "fuel")
	result, err := program.Call("get_fuel")
	require.NoError(err)
	require.LessOrEqual(into[uint64](result), program.Runtime.DefaultGas)
}
