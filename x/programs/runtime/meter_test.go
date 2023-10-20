// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v13"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"
)

func TestInfiniteLoop(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// infinite loop
	wasm, err := wasmtime.Wat2Wasm(`
	(module
	  (func (export "get_guest")
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

	_, err = runtime.Call(ctx, "get")
	var trap *wasmtime.Trap
	require.ErrorAs(err, &trap)
	require.ErrorContains(trap, "wasm trap: all fuel consumed")
}

func TestMetering(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// example has 2 ops codes and should cost 2 units
	wasm, err := wasmtime.Wat2Wasm(`
	(module $test
	(type (;0;) (func (result i32)))
	(export "get_guest" (func 0))
	(func (;0;) (type 0) (result i32)
		(local i32)
		i32.const 1
	  )
	)
	`)
	require.NoError(err)
	maxUnits := uint64(20)
	cfg, err := NewConfigBuilder().
		WithLimitMaxMemory(1 * MemoryPageSize). // 1 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, NoSupportedImports)
	err = runtime.Initialize(ctx, wasm, maxUnits)
	require.NoError(err)

	require.Equal(runtime.Meter().GetBalance(), maxUnits)
	for i := 0; i < 10; i++ {
		_, err = runtime.Call(ctx, "get")
		require.NoError(err)
	}
	require.Equal(runtime.Meter().GetBalance(), uint64(0))
}

func TestMeterAfterStop(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// example has 2 ops codes and should cost 2 units
	wasm, err := wasmtime.Wat2Wasm(`
	(module $test
	(type (;0;) (func (result i32)))
	(export "get_guest" (func 0))
	(func (;0;) (type 0) (result i32)
		(local i32)
		i32.const 1
	  )
	) 
	`)
	require.NoError(err)
	maxUnits := uint64(20)
	cfg, err := NewConfigBuilder().
		WithLimitMaxMemory(1 * MemoryPageSize). // 1 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, NoSupportedImports)
	err = runtime.Initialize(ctx, wasm, maxUnits)
	require.NoError(err)

	// spend 2 units
	_, err = runtime.Call(ctx, "get")
	require.NoError(err)
	// stop engine
	runtime.Stop()
	_, err = runtime.Call(ctx, "get")
	var trap *wasmtime.Trap
	require.ErrorAs(err, &trap)
	// ensure meter is still operational
	require.Equal(runtime.Meter().GetBalance(), maxUnits-2)
	_, err = runtime.Meter().AddUnits(2)
	require.NoError(err)
	require.Equal(runtime.Meter().GetBalance(), maxUnits)
}
