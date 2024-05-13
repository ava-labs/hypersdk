// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"context"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	v2 "github.com/ava-labs/hypersdk/x/programs/v2"
	"github.com/near/borsh-go"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

type testLoader struct {
}

func (testLoader) GetProgramBytes(programID ids.ID) ([]byte, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, "token.wasm"))
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
		testLoader{})

	saList := v2.NewSimpleStateAccessList([][]byte{{0}, {47, 0}, {47, 1}, {47, 2}}, [][]byte{{47, 0}, {47, 1}, {47, 2}})

	state := newTestDB()
	programID := ids.GenerateTestID()
	// all arguments are smart-pointers so this is a bit of a hack
	finished, err := runtime.CallProgram(ctx, &v2.CallInfo{ProgramID: programID, State: state, StateAccessList: saList, FunctionName: "init", Params: nil, Fuel: 1000000})
	require.Equal(nil, finished)
	require.NoError(err)

	supplyBytes, err := runtime.CallProgram(ctx, &v2.CallInfo{ProgramID: programID, State: state, StateAccessList: saList, FunctionName: "get_total_supply", Params: nil, Fuel: 1000000})
	require.NoError(err)
	expected, err := borsh.Serialize(123456789)
	require.NoError(err)
	require.Equal(expected, supplyBytes)

}
