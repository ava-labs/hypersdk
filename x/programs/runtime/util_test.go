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
	Runtime      *WasmRuntime
	StateManager StateManager
	DefaultGas   uint64
	DefaultValue uint64
}

func (t *testRuntime) AddProgram(programID ids.ID, programName string) {
	t.StateManager.(test.StateManager).ProgramsMap[programID] = programName
}

func (t *testRuntime) CallProgram(callContext Context, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		t.Context,
		&CallInfo{
			Program:      callContext.Program,
			Actor:        callContext.Actor,
			Height:       callContext.Height,
			Timestamp:    callContext.Timestamp,
			State:        t.StateManager,
			FunctionName: function,
			Params:       test.SerializeParams(params...),
			Fuel:         t.DefaultGas,
			Value:        t.DefaultValue,
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
				logging.NoLog{}),
			StateManager: test.StateManager{
				ProgramsMap: map[ids.ID]string{id: program},
				AccountMap:  map[codec.Address]ids.ID{account: id},
				Balances:    map[codec.Address]uint64{},
				Mu:          test.NewTestDB(),
			},
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
		Context{
			Program:   t.Address,
			Actor:     codec.CreateAddress(0, ids.GenerateTestID()),
			Height:    0,
			Timestamp: 0,
		},
		function,
		params...)
}

func (t *testProgram) CallWithActor(actor codec.Address, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		Context{
			Program:   t.Address,
			Actor:     actor,
			Height:    0,
			Timestamp: 0,
		},
		function,
		params...)
}

func (t *testProgram) CallWithHeight(height uint64, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		Context{
			Program:   t.Address,
			Actor:     codec.CreateAddress(0, ids.GenerateTestID()),
			Height:    height,
			Timestamp: 0,
		},
		function,
		params...)
}

func (t *testProgram) CallWithTimestamp(timestamp uint64, function string, params ...interface{}) ([]byte, error) {
	return t.Runtime.CallProgram(
		Context{
			Program:   t.Address,
			Actor:     codec.CreateAddress(0, ids.GenerateTestID()),
			Height:    0,
			Timestamp: timestamp,
		},
		function,
		params...)
}

func into[T any](data []byte) T {
	result, err := Deserialize[T](data)
	if err != nil {
		panic(err.Error())
	}
	return *result
}
