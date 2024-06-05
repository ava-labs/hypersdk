// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"github.com/ava-labs/hypersdk/codec"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

type testRuntime struct {
	Context    context.Context
	Runtime    *WasmRuntime
	StateDB    state.Mutable
	DefaultGas uint64
}

func (t *testRuntime) CallProgram(program ProgramInfo, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Context,
		&CallInfo{
			Program:      program,
			State:        t.StateDB,
			FunctionName: function,
			Params:       test.SerializeParams(params...),
			Fuel:         t.DefaultGas,
		})
}

func newTestProgram(ctx context.Context, program string) *testProgram {
	id := ids.GenerateTestID()
	account := codec.CreateAddress(0, id)
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
		Info: ProgramInfo{ID: id, Account: account},
	}
}

type testProgram struct {
	Runtime *testRuntime
	Info    ProgramInfo
}

func (t *testProgram) Call(function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Info,
		function,
		params...)
}
