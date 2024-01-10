// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package memory

import (
	"fmt"
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/tests"
)

func TestMemory(t *testing.T) {
	require := require.New(t)
	mem := newTestMemory(t)

	capacity, err := mem.Capacity()
	require.NoError(err)
	require.Equal(uint32(17*MemoryPageSize), capacity)

	// write to memory
	bytes := make([]byte, 2)
	s, err := WriteBytes(mem, bytes)
	require.NoError(err)

	readBytes, err := mem.Load(s)
	require.NoError(err)
	require.Equal(bytes, readBytes)
}

func newTestMemory(t *testing.T) Memory {
	require := require.New(t)
	wasmBytes := tests.ReadFixture(t, "../../tests/fixture/memory.wasm")

	// create new instance
	eng := engine.New(engine.NewConfig())
	store := engine.NewStore(eng, engine.NewStoreConfig())
	err := store.AddUnits(3000000)
	require.NoError(err)
	mod, err := wasmtime.NewModule(store.GetEngine(), wasmBytes)
	require.NoError(err)
	inst, err := wasmtime.NewInstance(store.Get(), mod, nil)
	require.NoError(err)
	// get alloc export func
	alloc := inst.GetExport(store.Get(), AllocFnName)
	require.NotNil(alloc)
	allocFunc := alloc.Func()
	require.NotNil(allocFunc)

	// get memory export func
	wmem := inst.GetExport(store.Get(), MemoryFnName).Memory()
	require.NotNil(wmem)
	mem := NewWasmTimeMemory(wmem, func(i int32) (uint32, error) {
		result, err := allocFunc.Call(store.Get(), i)
		if err != nil {
			return 0, err
		}

		addr, ok := result.(int32)
		if !ok {
			return 0, fmt.Errorf("%w: invalid result type: %T", ErrInvalidType, result)
		}
		if addr < 0 {
			return 0, ErrUnderflow
		}

		return uint32(addr), nil
	}, store.Get())
	require.NotNil(mem)
	return mem
}
