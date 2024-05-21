// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"

	v2 "github.com/ava-labs/hypersdk/x/programs/v2"
)

type tokenLoader struct{}

func (tokenLoader) GetProgramBytes(_ ids.ID) ([]byte, error) {
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
		logging.NoLog{},
		tokenLoader{})

	state := newTestDB()
	programID := ids.GenerateTestID()
	finished, err := runtime.CallProgram(ctx, &v2.CallInfo{ProgramID: programID, State: state, FunctionName: "init", Params: nil, Fuel: 1000000})
	require.NoError(err)
	require.Equal([]byte{}, finished)

	supplyBytes, err := runtime.CallProgram(ctx, &v2.CallInfo{ProgramID: programID, State: state, FunctionName: "get_total_supply", Params: nil, Fuel: 1000000})
	require.NoError(err)
	expected, err := borsh.Serialize(123456789)
	require.NoError(err)
	require.Equal(expected, supplyBytes)
}
