// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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

	rt := newTestRuntime(ctx)
	program, err := rt.newTestProgram("simple")
	require.NoError(err)

	for i := 0; i < b.N; i++ {
		result, err := program.Call("get_value")
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

func TestRuntimeCallProgramBasicAttachValue(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	program, err := rt.newTestProgram("simple")
	require.NoError(err)

	actor := codec.CreateAddress(0, ids.GenerateTestID())
	program.Runtime.StateManager.(TestStateManager).Balances[actor] = 10

	actorBalance, err := program.Runtime.StateManager.GetBalance(context.Background(), actor)
	require.NoError(err)
	require.Equal(uint64(10), actorBalance)

	// calling a program with a value transfers that amount from the caller to the program
	result, err := program.WithActor(actor).WithValue(4).Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	actorBalance, err = program.Runtime.StateManager.GetBalance(context.Background(), actor)
	require.NoError(err)
	require.Equal(uint64(6), actorBalance)

	programBalance, err := program.Runtime.StateManager.GetBalance(context.Background(), program.Address)
	require.NoError(err)
	require.Equal(uint64(4), programBalance)
}

func TestRuntimeCallProgramBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	program, err := rt.newTestProgram("simple")
	require.NoError(err)

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

	rt := newTestRuntime(ctx)
	program, err := rt.newTestProgram("return_complex_type")
	require.NoError(err)

	result, err := program.Call("get_value")
	require.NoError(err)
	require.Equal(ComplexReturn{Program: program.Address, MaxUnits: 1000}, into[ComplexReturn](result))
}
