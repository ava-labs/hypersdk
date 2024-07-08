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
	Context      context.Context
	callContext  CallContext
	StateManager StateManager
}

func (t *testRuntime) WithStateManager(manager StateManager) *testRuntime {
	t.callContext = t.callContext.WithStateManager(manager)
	return t
}

func (t *testRuntime) WithActor(address codec.Address) *testRuntime {
	t.callContext = t.callContext.WithActor(address)
	return t
}

func (t *testRuntime) WithFunction(s string) *testRuntime {
	t.callContext = t.callContext.WithFunction(s)
	return t
}

func (t *testRuntime) WithProgram(address codec.Address) *testRuntime {
	t.callContext = t.callContext.WithProgram(address)
	return t
}

func (t *testRuntime) WithFuel(u uint64) *testRuntime {
	t.callContext = t.callContext.WithFuel(u)
	return t
}

func (t *testRuntime) WithParams(bytes []byte) *testRuntime {
	t.callContext = t.callContext.WithParams(bytes)
	return t
}

func (t *testRuntime) WithHeight(height uint64) *testRuntime {
	t.callContext = t.callContext.WithHeight(height)
	return t
}

func (t *testRuntime) WithActionID(actionID ids.ID) *testRuntime {
	t.callContext = t.callContext.WithActionID(actionID)
	return t
}

func (t *testRuntime) WithTimestamp(ts uint64) *testRuntime {
	t.callContext = t.callContext.WithTimestamp(ts)
	return t
}

func (t *testRuntime) AddProgram(programID ids.ID, programName string) {
	t.StateManager.(test.StateManager).ProgramsMap[programID] = programName
}

func (t *testRuntime) CallProgram(program codec.Address, function string, params ...interface{}) ([]byte, error) {
	return t.callContext.CallProgram(
		t.Context,
		&CallInfo{
			Program:      program,
			State:        t.StateManager,
			FunctionName: function,
			Params:       test.SerializeParams(params...),
		})
}

func newTestProgram(ctx context.Context, program string) *testProgram {
	id := ids.GenerateTestID()
	account := codec.CreateAddress(0, id)
	return &testProgram{
		Runtime: &testRuntime{
			Context: ctx,
			callContext: NewRuntime(
				NewConfig(),
				logging.NoLog{}).WithDefaults(&CallInfo{Fuel: 10000000}),
			StateManager: test.StateManager{ProgramsMap: map[ids.ID]string{id: program}, AccountMap: map[codec.Address]ids.ID{account: id}, Mu: test.NewTestDB()},
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
		function,
		params...)
}

func (t *testProgram) WithStateManager(manager StateManager) *testProgram {
	t.Runtime = t.Runtime.WithStateManager(manager)
	return t
}

func (t *testProgram) WithActor(address codec.Address) *testProgram {
	t.Runtime = t.Runtime.WithActor(address)
	return t
}

func (t *testProgram) WithFunction(s string) *testProgram {
	t.Runtime = t.Runtime.WithFunction(s)
	return t
}

func (t *testProgram) WithProgram(address codec.Address) *testProgram {
	t.Runtime = t.Runtime.WithProgram(address)
	return t
}

func (t *testProgram) WithFuel(u uint64) *testProgram {
	t.Runtime = t.Runtime.WithFuel(u)
	return t
}

func (t *testProgram) WithParams(bytes []byte) *testProgram {
	t.Runtime = t.Runtime.WithParams(bytes)
	return t
}

func (t *testProgram) WithHeight(height uint64) *testProgram {
	t.Runtime = t.Runtime.WithHeight(height)
	return t
}

func (t *testProgram) WithActionID(actionID ids.ID) *testProgram {
	t.Runtime = t.Runtime.WithActionID(actionID)
	return t
}

func (t *testProgram) WithTimestamp(ts uint64) *testProgram {
	t.Runtime = t.Runtime.WithTimestamp(ts)
	return t
}

func into[T any](data []byte) T {
	result, err := Deserialize[T](data)
	if err != nil {
		panic(err.Error())
	}
	return *result
}
