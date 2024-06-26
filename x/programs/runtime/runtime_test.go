// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func BenchmarkRuntimeCallProgramBasic(b *testing.B) {
	require := require.New(b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "simple")

	for i := 0; i < b.N; i++ {
		result, err := program.Call("get_value")
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

func TestRuntimeCallProgramBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "simple")

	result, err := program.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))
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
	require.Equal(ComplexReturn{Program: program.Address, MaxUnits: 1000}, into[ComplexReturn](result))
}

func TestContextInjection(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	program := newTestProgram(ctx, "context_injection")

	_, err := program.Call("can_change_timestamp")
	require.NoError(err)
	_, err = program.CallWithTimestamp(1, "can_change_timestamp")
	require.NoError(err)

	_, err = program.Call("can_change_height")
	require.NoError(err)
	_, err = program.CallWithHeight(1, "can_change_height")
	require.NoError(err)

	_, err = program.Call("can_change_actor")
	require.NoError(err)
	_, err = program.CallWithActor(codec.CreateAddress(0, ids.GenerateTestID()), "can_change_actor")
	require.NoError(err)
}
