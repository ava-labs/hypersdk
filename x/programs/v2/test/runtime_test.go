// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"context"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	v2 "github.com/ava-labs/hypersdk/x/programs/v2"
	"github.com/near/borsh-go"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

type tokenLoader struct {
}

func (tokenLoader) GetProgramBytes(programID ids.ID) ([]byte, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "token.wasm"))
}

type counterLoader struct {
}

func (counterLoader) GetProgramBytes(programID ids.ID) ([]byte, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "counter.wasm"))
}

type getExternalArgs struct {
	Target   ids.ID
	MaxUnits int64
}

type initialize struct {
	Address codec.Address
}

func TestProgramCall(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := v2.NewRuntime(
		v2.NewConfig(),
		logging.NoLog{}, &testDB{
			db: memdb.New(),
		},
		tokenLoader{})

	saList := v2.NewSimpleStateAccessList([][]byte{}, [][]byte{})

	state := newTestDB()
	programID := ids.GenerateTestID()
	address := codec.CreateAddress(0, ids.GenerateTestID())

	paramBytes, err := borsh.Serialize(initialize{
		Address: address,
	})
	require.NoError(err)
	finished, err := runtime.CallProgram(
		ctx,
		&v2.CallInfo{
			ProgramID:       programID,
			State:           state,
			StateAccessList: saList,
			FunctionName:    "initialize_address",
			Params:          paramBytes,
			Fuel:            1000000})
	require.NoError(err)
	require.Equal([]byte{}, finished)

	paramBytes, err = borsh.Serialize(getExternalArgs{
		Target:   ids.GenerateTestID(),
		MaxUnits: 100000,
	})
	require.NoError(err)
	finished, err = runtime.CallProgram(
		ctx,
		&v2.CallInfo{
			ProgramID:       programID,
			State:           state,
			StateAccessList: saList,
			FunctionName:    "get_value_external",
			Params:          paramBytes,
			Fuel:            1000000})
	require.NoError(err)
	require.Equal([]byte{}, finished)
}

func TestSimpleCall(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := v2.NewRuntime(
		v2.NewConfig(),
		logging.NoLog{}, &testDB{
			db: memdb.New(),
		},
		tokenLoader{})

	saList := v2.NewSimpleStateAccessList([][]byte{}, [][]byte{})

	state := newTestDB()
	programID := ids.GenerateTestID()
	finished, err := runtime.CallProgram(ctx, &v2.CallInfo{ProgramID: programID, State: state, StateAccessList: saList, FunctionName: "init", Params: nil, Fuel: 1000000})
	require.NoError(err)
	require.Equal([]byte{}, finished)

	supplyBytes, err := runtime.CallProgram(ctx, &v2.CallInfo{ProgramID: programID, State: state, StateAccessList: saList, FunctionName: "get_total_supply", Params: nil, Fuel: 1000000})
	require.NoError(err)
	expected, err := borsh.Serialize(123456789)
	require.NoError(err)
	require.Equal(expected, supplyBytes)

	paramBytes, err := borsh.Serialize(getExternalArgs{
		Target:   ids.GenerateTestID(),
		MaxUnits: 100000,
	})
	require.NoError(err)
	supplyBytes, err = runtime.CallProgram(ctx,
		&v2.CallInfo{ProgramID: programID, State: state, StateAccessList: saList, FunctionName: "get_total_supply_external", Params: paramBytes, Fuel: 1000000})
	require.NoError(err)
	require.Equal(expected, supplyBytes)
}
