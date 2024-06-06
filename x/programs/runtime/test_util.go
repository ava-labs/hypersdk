// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

type testRuntime struct {
	Context    context.Context
	Runtime    *WasmRuntime
	StateDB    StateLoader
	DefaultGas uint64
}

func (t *testRuntime) CallProgram(program codec.Address, actor codec.Address, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Context,
		&CallInfo{
			Program:      program,
			Actor:        actor,
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
				test.ProgramLoader{ProgramName: program}),
			StateDB:    test.StateLoader{Mu: test.NewTestDB()},
			DefaultGas: 10000000,
		},
		Address: account,
	}
}

type testProgram struct {
	Runtime *testRuntime
	Address codec.Address
}

func (t *testProgram) Call(function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Address,
		codec.CreateAddress(0, ids.GenerateTestID()),
		function,
		params...)
}

func (t *testProgram) CallWithActor(actor codec.Address, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Address,
		actor,
		function,
		params...)
}
