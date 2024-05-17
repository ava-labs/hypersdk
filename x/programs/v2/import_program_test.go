// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"context"
	"github.com/ava-labs/hypersdk/x/programs/v2/test"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"
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
	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get_value", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(0)
	require.NoError(err)
	require.Equal(expected, result)

	params := struct {
		Program  ids.ID
		MaxUnits uint64
	}{
		Program:  programID,
		MaxUnits: 1000000,
	}
	paramBytes, err := borsh.Serialize(params)
	require.NoError(err)
	result, err = runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get_value_external", Params: paramBytes, Fuel: 10000000})
	require.NoError(err)
	require.Equal(expected, result)
}
