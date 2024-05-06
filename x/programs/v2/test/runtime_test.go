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

	bytes, err = borsh.Serialize(ctx)
	require.NoError(err)

	// all arguments are smart-pointers so this is a bit of a hack
	_, err = runtime.CallProgram(ctx, &v2.CallInfo{Program: programID, FunctionName: "init", Params: bytes, MaxUnits: 10000})
	require.NoError(err)
}
