// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"context"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/x/programs/program"
	v2 "github.com/ava-labs/hypersdk/x/programs/v2"
	"github.com/near/borsh-go"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

func TestSimpleCall(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := v2.NewRuntime(logging.NoLog{}, &testDB{
		db: memdb.New(),
	}, v2.NewConfig())

	dir, err := os.Getwd()
	require.NoError(err)
	bytes, err := os.ReadFile(filepath.Join(dir, "token.wasm"))
	require.NoError(err)

	programCtx := program.Context{ProgramID: ids.GenerateTestID()}
	programID := v2.ProgramID(programCtx.ProgramID.String())
	err = runtime.AddProgram(programID, bytes)
	require.NoError(err)

	bytes, err = borsh.Serialize(programCtx)
	require.NoError(err)

	saList := v2.NewSimpleStateAccessList([][]byte{{0}, {47, 0}, {47, 1}, {47, 2}}, [][]byte{{47, 0}, {47, 1}, {47, 2}})

	state := newTestDB()
	// all arguments are smart-pointers so this is a bit of a hack
	finished, err := runtime.CallProgram(ctx, &v2.CallInfo{Program: programID, State: state, StateAccessList: saList, FunctionName: "init", Params: bytes, MaxUnits: 1000000})
	require.Equal([]byte{1, 0, 0, 0}, finished)
	require.NoError(err)

	supplyBytes, err := runtime.CallProgram(ctx, &v2.CallInfo{Program: programID, State: state, StateAccessList: saList, FunctionName: "get_total_supply", Params: bytes, MaxUnits: 1000000})
	require.NoError(err)
	expected, err := borsh.Serialize(123456789)
	require.NoError(err)
	require.Equal(expected, supplyBytes)

}
