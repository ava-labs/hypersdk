// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/stretchr/testify/require"
)

func TestStop(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// infinite loop
	wasm, err := wasmtime.Wat2Wasm(`
	(module
	  (func (export "run_guest")
	    (loop
	      br 0)
	  )
	)
	`)
	require.NoError(err)
	maxUnits := uint64(10000)
	cfg, err := NewConfigBuilder().
		WithLimitMaxMemory(1 * MemoryPageSize). // 1 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, NoSupportedImports)
	err = runtime.Initialize(ctx, wasm, maxUnits)
	require.NoError(err)
	// stop the runtime
	runtime.Stop()

	_, err = runtime.Call(ctx, "run")
	require.ErrorIs(err, ErrTrapUnreachableCodeReached)
	// ensure no fees were consumed
	require.Equal(runtime.Meter().GetBalance(), maxUnits)
}

func TestCallParams(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// add param[0] + param[1]
	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (func $add_guest (param $a i32) (param $b i32) (result i32)
        (i32.add (local.get $a) (local.get $b))
      )
	  (export "add_guest" (func $add_guest))
    )
	`)
	require.NoError(err)
	maxUnits := uint64(10000)
	cfg, err := NewConfigBuilder().
		WithLimitMaxMemory(1 * MemoryPageSize). // 1 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, NoSupportedImports)
	err = runtime.Initialize(ctx, wasm, maxUnits)
	require.NoError(err)

	resp, err := runtime.Call(ctx, "add", uint64(10), uint64(10))
	require.NoError(err)
	require.Equal(uint64(20), resp[0])

	// pass 3 params when 2 are expected.
	_, err = runtime.Call(ctx, "add", uint64(10), uint64(10), uint64(10))
	require.ErrorIs(err, ErrInvalidParamCount)
}
