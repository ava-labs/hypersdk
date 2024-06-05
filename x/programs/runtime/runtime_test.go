// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestRuntimeCallProgramBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "simple")
	result, err := program.Call("get_value")
	require.NoError(err)
	expected, err := borsh.Serialize(0)
	require.NoError(err)
	require.Equal(expected, result)
}

type ComplexReturn struct {
	Program  ids.ID
	MaxUnits uint64
}

func TestRuntimeCallProgramComplexReturn(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "return_complex_type")
	result, err := program.Call("get_value")
	require.NoError(err)
	require.Equal(test.SerializeParams(ComplexReturn{Program: program.ProgramID, MaxUnits: 1000}), result)
}
