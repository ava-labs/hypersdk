// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestImportProgramCallProgram(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "call_program"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()
	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "simple_call", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(0)
	require.NoError(err)
	require.Equal(expected, result)

	params := struct {
		Program  ids.ID
		MaxUnits int64
	}{
		Program:  programID,
		MaxUnits: 1000000,
	}
	paramBytes, err := borsh.Serialize(params)
	require.NoError(err)
	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "simple_call_external", Params: paramBytes, Fuel: 10000000})
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramWithParam(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "call_program"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	expected, err := borsh.Serialize(uint64(1))
	require.NoError(err)

	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "call_with_param", Params: expected, Fuel: 10000000})
	require.NoError(err)
	require.Equal(expected, result)

	params := struct {
		Program  ids.ID
		MaxUnits uint64
		Value    uint64
	}{
		Program:  programID,
		MaxUnits: 1000000,
		Value:    1,
	}
	paramBytes, err := borsh.Serialize(params)
	require.NoError(err)
	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "call_with_param_external", Params: paramBytes, Fuel: 10000000})
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportProgramCallProgramWithParams(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "call_program"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	expected, err := borsh.Serialize(int64(3))
	require.NoError(err)

	paramBytes, err := borsh.Serialize(struct {
		Value1 int64
		Value2 int64
	}{
		Value1: 1,
		Value2: 2,
	})
	require.NoError(err)

	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "call_with_two_params", Params: paramBytes, Fuel: 10000000})
	require.NoError(err)
	require.Equal(expected, result)

	paramBytes, err = borsh.Serialize(struct {
		Program  ids.ID
		MaxUnits uint64
		Value1   int64
		Value2   int64
	}{
		Program:  programID,
		MaxUnits: 1000000,
		Value1:   1,
		Value2:   2,
	})
	require.NoError(err)
	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "call_with_two_params_external", Params: paramBytes, Fuel: 10000000})
	require.NoError(err)
	require.Equal(expected, result)
}

func TestImportGetRemainingFuel(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "fuel"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	startFuel := uint64(100000)
	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get_fuel", Params: nil, Fuel: startFuel})
	require.NoError(err)
	remaining := uint64(0)
	require.NoError(borsh.Deserialize(&remaining, result))
	require.LessOrEqual(remaining, startFuel)
}
