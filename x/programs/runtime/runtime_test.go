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

func TestRuntimeCallProgramBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "simple"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()
	result, err := runtime.CallProgram(ctx, &CallInfo{Program: ProgramInfo{ID: programID}, State: state, FunctionName: "get_value", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(0)
	require.NoError(err)
	require.Equal(expected, result)
}

type ComplexReturn struct {
	Program  ProgramInfo
	MaxUnits uint64
}

func TestRuntimeCallProgramComplexReturn(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "return_complex_type"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()
	result, err := runtime.CallProgram(ctx, &CallInfo{Program: ProgramInfo{ID: programID}, State: state, FunctionName: "get_value", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(ComplexReturn{Program: ProgramInfo{ID: programID}, MaxUnits: 1000})
	require.NoError(err)
	require.Equal(expected, result)
}
