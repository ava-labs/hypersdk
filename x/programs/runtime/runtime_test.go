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
	tests := []struct {
		name string
		fun  func(*testProgram, *require.Assertions)
	}{
		{
			name: "timestamp",
			fun: func(program *testProgram, require *require.Assertions) {
				result, err := program.CallWithTimestamp(1, "get_timestamp")
				require.NoError(err)
				require.Equal(uint64(1), into[uint64](result))
			},
		},
		{
			name: "height",
			fun: func(program *testProgram, require *require.Assertions) {
				result, err := program.CallWithHeight(1, "get_height")
				require.NoError(err)
				require.Equal(uint64(1), into[uint64](result))
			},
		},
		{
			name: "actor",
			fun: func(program *testProgram, require *require.Assertions) {
				result, err := program.CallWithActor(codec.CreateAddress(0, ids.GenerateTestID()), "get_actor")
				require.NoError(err)
				require.NotEqual(ids.Empty, into[ids.ID](result))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			program := newTestProgram(ctx, "context_injection")

			tt.fun(program, require)
		})
	}
}
