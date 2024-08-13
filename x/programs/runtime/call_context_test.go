/*
 * Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestCallContext(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	programID := ids.GenerateTestID()
	programAccount := codec.CreateAddress(0, programID)
	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
	).WithDefaults(
		CallInfo{
			State:   &test.StateManager{ProgramsMap: map[ids.ID]string{programID: "call_program"}, AccountMap: map[codec.Address]ids.ID{programAccount: programID}},
			Program: programAccount,
			Fuel:    1000000,
		})
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := r.WithActor(actor).CallProgram(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.NoError(err)
	require.Equal(actor, into[codec.Address](result))

	result, err = r.WithActor(codec.CreateAddress(2, ids.GenerateTestID())).CallProgram(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.NoError(err)
	require.NotEqual(actor, into[codec.Address](result))

	result, err = r.WithFuel(0).CallProgram(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.Equal(wasmtime.OutOfFuel, *err.(*wasmtime.Trap).Code())
	require.Nil(result)
}

func TestCallContextPreventOverwrite(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program0ID := ids.GenerateTestID()
	program0Address := codec.CreateAddress(0, program0ID)
	program1ID := ids.GenerateTestID()
	program1Address := codec.CreateAddress(1, program1ID)

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
	).WithDefaults(
		CallInfo{
			Program: program0Address,
			State:   &test.StateManager{ProgramsMap: map[ids.ID]string{program0ID: "call_program"}, AccountMap: map[codec.Address]ids.ID{program0Address: program0ID}},
			Fuel:    1000000,
		})

	// try to use a context that has a default program with a different program
	result, err := r.CallProgram(
		ctx,
		&CallInfo{
			Program:      program1Address,
			State:        &test.StateManager{ProgramsMap: map[ids.ID]string{program1ID: "call_program"}, AccountMap: map[codec.Address]ids.ID{program1Address: program1ID}},
			FunctionName: "actor_check",
		})
	require.ErrorIs(err, errCannotOverwrite)
	require.Nil(result)
}
