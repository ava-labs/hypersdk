// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestImportProgramCallProgram(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")

	result, err := program.Call("simple_call")
	require.NoError(err)
	require.Equal(int64(0), test.Into[int64](result))

	result, err = program.Call(
		"simple_call_external",
		program.Info, uint64(1000000))
	require.NoError(err)
	require.Equal(uint64(0), test.Into[uint64](result))
}

func TestImportProgramCallProgramWithParam(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")

	result, err := program.Call(
		"call_with_param",
		uint64(1))
	require.NoError(err)
	require.Equal(uint64(1), test.Into[uint64](result))

	result, err = program.Call(
		"call_with_param_external",
		program.Info, uint64(1000000), uint64(1))
	require.NoError(err)
	require.Equal(uint64(1), test.Into[uint64](result))
}

func TestImportProgramCallProgramWithParams(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")

	result, err := program.Call(
		"call_with_two_params",
		1, 2)
	require.NoError(err)
	require.Equal(uint64(3), test.Into[uint64](result))

	result, err = program.Call(
		"call_with_two_params_external",
		program.Info, 1000000, 1, 2)
	require.NoError(err)
	require.Equal(uint64(3), test.Into[uint64](result))
}

func TestImportGetRemainingFuel(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "fuel")

	result, err := program.Call("get_fuel")
	require.NoError(err)
	require.LessOrEqual(test.Into[uint64](result), program.Runtime.DefaultGas)
}

func TestImportProgramCallProgramActor(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := program.CallWithActor(actor, "actor_check")
	require.NoError(err)
	require.Equal(actor, test.Into[codec.Address](result))
}

func TestImportProgramCallProgramActorChange(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "call_program")
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	// the actor changes to the calling program's account
	result, err := program.CallWithActor(
		actor,
		"actor_check_external",
		program.Info, uint64(200000))
	require.NoError(err)
	require.Equal(program.Info.Account, test.Into[codec.Address](result))
}
