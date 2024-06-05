package runtime

import (
	"context"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

type testRuntime struct {
	Context    context.Context
	Runtime    *WasmRuntime
	StateDB    state.Mutable
	DefaultGas uint64
}

func (t *testRuntime) CallProgram(programID ids.ID, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Context,
		&CallInfo{
			ProgramID:    programID,
			State:        t.StateDB,
			FunctionName: function,
			Params:       test.SerializeParams(params...),
			Fuel:         t.DefaultGas,
		})
}

func newTestProgram(ctx context.Context, program string) *testProgram {
	return &testProgram{
		Runtime: &testRuntime{
			Context: ctx,
			Runtime: NewRuntime(
				NewConfig(),
				logging.NoLog{},
				test.Loader{ProgramName: program}),
			StateDB:    test.NewTestDB(),
			DefaultGas: 10000000,
		},
		ProgramID: ids.GenerateTestID(),
	}
}

type testProgram struct {
	Runtime   *testRuntime
	ProgramID ids.ID
}

func (t *testProgram) Call(function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.ProgramID,
		function,
		params...)
}
