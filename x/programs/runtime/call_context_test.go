// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestCallContext(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	programID := ids.GenerateTestID()
	programAccount := codec.CreateAddress(0, programID)
	stringedID := string(programID[:])
	testStateManager := &TestStateManager{
		ProgramsMap: map[string][]byte{},
		AccountMap:  map[codec.Address]string{programAccount: stringedID},
	}
	err := testStateManager.CompileAndSetProgram(ProgramID(stringedID), "call_program")
	require.NoError(err)

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
	).WithDefaults(
		CallInfo{
			State:   testStateManager,
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
	stringedID0 := string(program0ID[:])

	testStateManager := &TestStateManager{
		ProgramsMap: map[string][]byte{},
		AccountMap:  map[codec.Address]string{program0Address: stringedID0},
	}

	err := testStateManager.CompileAndSetProgram(ProgramID(stringedID0), "call_program")
	require.NoError(err)

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
	).WithDefaults(
		CallInfo{
			Program: program0Address,
			State:   testStateManager,
			Fuel:    1000000,
		})

	stringedID1 := string(program1ID[:])
	testStateManager1 := &TestStateManager{
		ProgramsMap: map[string][]byte{},
		AccountMap:  map[codec.Address]string{program1Address: stringedID1},
	}
	err = testStateManager1.CompileAndSetProgram(ProgramID(stringedID1), "call_program")
	require.NoError(err)

	// try to use a context that has a default program with a different program
	result, err := r.CallProgram(
		ctx,
		&CallInfo{
			Program:      program1Address,
			State:        testStateManager1,
			FunctionName: "actor_check",
		})
	require.ErrorIs(err, errCannotOverwrite)
	require.Nil(result)
}
