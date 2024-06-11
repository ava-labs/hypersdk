// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestRuntimeCallProgramBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "simple")

	result, err := program.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), test.Into[uint64](result))
}

type ComplexReturn struct {
	Program  codec.Address
	MaxUnits uint64
}

func TestRuntimeCallProgramComplexReturn(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "return_complex_type")

	result, err := program.Call("get_value")
	require.NoError(err)
	require.Equal(ComplexReturn{Program: program.Address, MaxUnits: 1000}, test.Into[ComplexReturn](result))
}
