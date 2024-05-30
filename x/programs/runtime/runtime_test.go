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

func BenchmarkRuntimeCallProgramSimple(b *testing.B) {
	require := require.New(b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime, err := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "simple"})
	require.NoError(err)

	for i := 0; i < b.N; i++ {
		_, err := runtime.CallProgram(
			ctx,
			&CallInfo{
				ProgramID:    ids.Empty,
				State:        nil,
				FunctionName: "get_value",
				Params:       nil,
				Fuel:         10000000,
			})
		require.NoError(err)
	}
}

func BenchmarkRuntimeCallProgramComplex(b *testing.B) {
	require := require.New(b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime, err := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "return_complex_type"})
	require.NoError(err)
	for i := 0; i < b.N; i++ {
		_, err := runtime.CallProgram(
			ctx,
			&CallInfo{
				ProgramID:    ids.Empty,
				State:        nil,
				FunctionName: "get_value",
				Params:       nil,
				Fuel:         10000000,
			})
		require.NoError(err)
	}
}

func TestRuntimeCallProgramBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime, err := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "simple"})
	require.NoError(err)
	state := test.NewTestDB()
	programID := ids.GenerateTestID()
	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get_value", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(0)
	require.NoError(err)
	require.Equal(expected, result)
}

type ComplexReturn struct {
	Program  ids.ID
	MaxUnits uint64
}

func TestRuntimeCallProgramComplexReturn(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime, err := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "return_complex_type"})
	require.NoError(err)
	state := test.NewTestDB()
	programID := ids.GenerateTestID()
	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get_value", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(ComplexReturn{Program: programID, MaxUnits: 1000})
	require.NoError(err)
	require.Equal(expected, result)
}
