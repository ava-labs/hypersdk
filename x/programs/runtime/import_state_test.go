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

func TestImportStatePutGet(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "state_access"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	valueBytes, err := borsh.Serialize(int64(10))
	require.NoError(err)

	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "put", Params: valueBytes, Fuel: 10000000})
	require.NoError(err)
	require.Nil(result)

	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get", Params: nil, Fuel: 10000000})
	require.NoError(err)
	require.Equal(append([]byte{1}, valueBytes...), result)
}

func TestImportStateRemove(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "state_access"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	valueBytes, err := borsh.Serialize(int64(10))
	require.NoError(err)

	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "put", Params: valueBytes, Fuel: 10000000})
	require.NoError(err)
	require.Nil(result)

	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "delete", Params: nil, Fuel: 10000000})
	require.NoError(err)
	require.Equal(append([]byte{1}, valueBytes...), result)

	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get", Params: nil, Fuel: 10000000})
	require.NoError(err)
	require.Equal([]byte{0}, result)
}

func TestImportStateDeleteMissingKey(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "state_access"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "delete", Params: nil, Fuel: 10000000})
	require.NoError(err)
	require.Equal([]byte{0}, result)
}

func TestImportStateGetMissingKey(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "state_access"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()

	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get", Params: nil, Fuel: 10000000})
	require.NoError(err)
	require.Equal([]byte{0}, result)
}
