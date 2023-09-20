// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v12"
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
	cfg, err := NewConfigBuilder(maxUnits).
		WithLimitMaxMemory(1 * MemoryPageSize). // 1 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, NoImports)
	err = runtime.Initialize(ctx, wasm)
	require.NoError(err)
	// stop the runtime
	runtime.Stop()

	_, err = runtime.Call(ctx, "run")
	var trap *wasmtime.Trap
	require.ErrorAs(err, &trap)
	require.ErrorContains(trap, "wasm trap: interrupt")
	// ensure no fees were consumed
	require.Equal(runtime.Meter().GetBalance(), maxUnits)
}
