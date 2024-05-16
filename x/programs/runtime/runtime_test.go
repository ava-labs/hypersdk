// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

func TestStop(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// infinite loop
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		(func $run (param i32)
			(loop br 0)
		)
		(func $alloc (param i32) (result i32)
	      i32.const 0
	 	)
		(export "memory" (memory 0))
		(export "run_guest" (func $run))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)
	maxUnits := uint64(10000)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(ctx, programContext, wasm, maxUnits)
	require.NoError(err)
	// stop the runtime
	runtime.Stop()

	_, err = runtime.Call(ctx, "run", programContext)
	require.ErrorIs(err, program.ErrTrapInterrupt)
	// ensure no fees were consumed
	balance, err := runtime.Meter().GetBalance()
	require.NoError(err)
	require.Equal(balance, maxUnits)
}

func TestCallParams(t *testing.T) {
	t.Skip("ignoring test for now")
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// add param[0] + param[1]
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		;; first argument is always the pointer to the context
		(func $add (param i32 i64 i64) (result i64)
			(i64.add local.get 1 local.get 2)
		)
		(func $alloc (param i32) (result i32)
	      i32.const 0
	 	)
		(export "memory" (memory 0))
		(export "add_guest" (func $add))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)
	maxUnits := uint64(10000)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(ctx, programContext, wasm, maxUnits)
	require.NoError(err)

	arg := uint32(10)

	// all arguments are smart-pointers so this is a bit of a hack
	resp, err := runtime.Call(ctx, "add", programContext, arg, arg)
	// convert the `resp` byte-slice into an int64
	fmt.Fprintf(os.Stderr, "resp: %v\n", resp)

	require.NoError(err)
	require.Equal(arg+arg, binary.LittleEndian.Uint64(resp))

	// pass 3 params when 2 are expected.
	_, err = runtime.Call(ctx, "add", programContext, 10, 10, 10)
	require.ErrorIs(err, program.ErrInvalidParamCount)
}

func TestInfiniteLoop(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// infinite loop
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		(func $run (param i32)
			(loop br 0)
		)
		(func $alloc (param i32) (result i32)
	      i32.const 0
	 	)
		(export "memory" (memory 0))
		(export "run_guest" (func $run))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)
	maxUnits := uint64(10000)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(ctx, programContext, wasm, maxUnits)
	require.NoError(err)

	_, err = runtime.Call(ctx, "run", programContext)
	require.ErrorIs(err, program.ErrTrapOutOfFuel)
}

func TestMetering(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// example has 2 ops codes and should cost 2 units
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		(func $get (param i32) (result i32)
			i32.const 0
		)
		(func $alloc (param i32) (result i32)
			i32.const 0
		)
		(export "memory" (memory 0))
		(export "get_guest" (func $get))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)
	maxUnits := uint64(20)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(ctx, programContext, wasm, maxUnits)
	require.NoError(err)
	balance, err := runtime.Meter().GetBalance()
	require.NoError(err)

	require.Equal(balance, maxUnits)
	// spend 2 units for alloc and 2 units for get_guest
	for i := 0; i < 5; i++ {
		_, err = runtime.Call(ctx, "get", programContext)
		require.NoError(err)
	}
	balance, err = runtime.Meter().GetBalance()
	require.NoError(err)
	require.Zero(balance)
}

func TestMeterAfterStop(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// example has 2 ops codes and should cost 2 units
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		(func $get (param i32) (result i32)
			i32.const 0
		)
		(func $alloc (param i32) (result i32)
			i32.const 0
		)
		(export "memory" (memory 0))
		(export "get_guest" (func $get))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)
	maxUnits := uint64(20)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(ctx, programContext, wasm, maxUnits)
	require.NoError(err)

	// spend 4 units
	_, err = runtime.Call(ctx, "get", programContext)
	require.NoError(err)
	// stop engine
	runtime.Stop()
	_, err = runtime.Call(ctx, "get", programContext)
	require.ErrorIs(err, program.ErrTrapInterrupt)
	// ensure meter is still operational
	balance, err := runtime.Meter().GetBalance()
	require.NoError(err)
	require.Equal(balance, maxUnits-4)
}

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
	maxUnits := uint64(1)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(context.Background(), programContext, wasm, maxUnits)
	require.ErrorContains(err, "memory minimum size of 2 pages exceeds memory limits") //nolint:forbidigo
}

func TestLimitMaxMemoryGrow(t *testing.T) {
	require := require.New(t)

	// we require an exported alloc function
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		(func $alloc (param i32) (result i32)
			i32.const 0
		)
		(export "memory" (memory 0))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)

	maxUnits := uint64(1)
	cfg := NewConfig().SetLimitMaxMemory(1 * program.MemoryPageSize)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(context.Background(), programContext, wasm, maxUnits)
	require.NoError(err)

	mem, err := runtime.Memory()
	require.NoError(err)
	length, err := mem.Len()
	require.NoError(err)
	require.Equal(uint32(0x10000), length)

	// attempt to grow memory to 2 pages which exceeds the limit
	_, err = mem.Grow(1)
	require.ErrorContains(err, "failed to grow memory by `1`") //nolint:forbidigo
}

func TestWriteExceedsLimitMaxMemory(t *testing.T) {
	require := require.New(t)

	// we require an exported alloc function
	wasm, err := wasmtime.Wat2Wasm(`
	(module
	  (func (result i32)
		(i32.const 42)
	  )
      (export "alloc" (func 0))
	  (memory 1) ;; 1 pages
	  (export "memory" (memory 0))
	)
	`)
	require.NoError(err)

	maxUnits := uint64(1)
	cfg := NewConfig()
	eng := engine.New(engine.NewConfig())
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(context.Background(), programContext, wasm, maxUnits)
	require.NoError(err)
	mem, err := runtime.Memory()
	require.NoError(err)
	maxMemory, err := mem.Len()
	require.NoError(err)

	bytes := utils.RandomBytes(int(maxMemory) + 1)
	err = mem.Write(0, bytes)
	require.ErrorIs(err, program.ErrOverflow)
}

func TestWithMaxWasmStack(t *testing.T) {
	require := require.New(t)
	wasm, err := wasmtime.Wat2Wasm(`
	(module
		(memory 1) ;; 1 pages
		(func $get (param i32) (result i32)
			i32.const 0
		)
		(func $alloc (param i32) (result i32)
			i32.const 0
		)
		(export "memory" (memory 0))
		(export "get_guest" (func $get))
		(export "alloc" (func $alloc))
	)
	`)
	require.NoError(err)

	maxUnits := uint64(4)
	ecfg, err := engine.NewConfigBuilder().
		WithMaxWasmStack(1000).
		Build()
	require.NoError(err)
	eng := engine.New(ecfg)
	cfg := NewConfig()
	runtime := New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)

	id := ids.GenerateTestID()
	programContext := &program.Context{
		ProgramID: id,
	}

	err = runtime.Initialize(context.Background(), programContext, wasm, maxUnits)
	require.NoError(err)
	_, err = runtime.Call(context.Background(), "get", programContext)
	require.NoError(err)

	// stack is ok for 1 call.
	ecfg, err = engine.NewConfigBuilder().
		WithMaxWasmStack(580).
		Build()
	require.NoError(err)
	eng = engine.New(ecfg)
	runtime = New(logging.NoLog{}, eng, host.NoSupportedImports, cfg)
	err = runtime.Initialize(context.Background(), programContext, wasm, maxUnits)
	require.NoError(err)
	// exceed the stack limit
	_, err = runtime.Call(context.Background(), "get", programContext)
	require.ErrorIs(err, program.ErrTrapStackOverflow)
}
