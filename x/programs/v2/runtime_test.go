package v2

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/v2/test"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"
	"testing"
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

	runtime := NewRuntime(
		NewConfig(),
		logging.NoLog{},
		test.Loader{ProgramName: "return_complex_type"})

	state := test.NewTestDB()
	programID := ids.GenerateTestID()
	result, err := runtime.CallProgram(ctx, &CallInfo{ProgramID: programID, State: state, FunctionName: "get_value", Params: nil, Fuel: 10000000})
	require.NoError(err)
	expected, err := borsh.Serialize(ComplexReturn{Program: programID, MaxUnits: 1000})
	require.NoError(err)
	require.Equal(expected, result)
}
