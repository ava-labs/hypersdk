// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/bytecodealliance/wasmtime-go/v12"
	"github.com/stretchr/testify/require"
)

var log = logging.NewLogger(
	"",
	logging.NewWrappedCore(
		logging.Info,
		os.Stderr,
		logging.Plain.ConsoleEncoder(),
	))

func TestLimitMaxMemory(t *testing.T) {
	require := require.New(t)

	// memory has a single page
	wasm, err := wasmtime.Wat2Wasm(`
	(module

	  (memory 2) ;; 2 pages
	  (export "memory" (memory 0))
	)
	`)
	require.NoError(err)

	// wasm defines 2 pages of memory but runtime set max 1 page
	maxFee := uint64(1)
	cfg, err := NewConfigBuilder(maxFee).
		WithLimitMaxMemory(1 * 64 * units.KiB). // 1 pages
		Build()
	require.NoError(err)
	runtime := New(log, cfg, nil)
	err = runtime.Initialize(context.Background(), wasm)
	require.ErrorContains(err, "memory minimum size of 2 pages exceeds memory limits")
}

func TestLimitMaxMemoryGrow(t *testing.T) {
	require := require.New(t)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
	
	  (memory 1) ;; 1 pages
	  (export "memory" (memory 0))
	)
	`)
	require.NoError(err)

	maxFee := uint64(1)
	cfg, err := NewConfigBuilder(maxFee).
		WithLimitMaxMemory(1 * 64 * units.KiB). // 2 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, nil)
	err = runtime.Initialize(context.Background(), wasm)
	require.NoError(err)

	length, err := runtime.Memory().Len()
	require.NoError(err)
	require.Equal(uint64(0x10000), length)

	// attempt to grow memory to 2 pages which exceeds the limit
	_, err = runtime.Memory().Grow(1)
	require.ErrorContains(err, "failed to grow memory by `1`")
}

func TestWriteExceedsLimitMaxMemory(t *testing.T) {
	require := require.New(t)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
	
	  (memory 1) ;; 1 pages
	  (export "memory" (memory 0))
	)
	`)
	require.NoError(err)

	maxFee := uint64(1)
	cfg, err := NewConfigBuilder(maxFee).
		WithLimitMaxMemory(1 * 64 * units.KiB). // 2 pages
		Build()
	require.NoError(err)
	runtime := New(logging.NoLog{}, cfg, nil)
	err = runtime.Initialize(context.Background(), wasm)
	require.NoError(err)
	maxMemory, err := runtime.Memory().Len()
	require.NoError(err)

	bytes := utils.RandomBytes(int(maxMemory) + 1)
	err = runtime.Memory().Write(0, bytes)
	require.Error(err, "write memory failed: invalid memory size")
}
