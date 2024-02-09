// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/tests"
)

func TestMemory(t *testing.T) {
	require := require.New(t)
	mem := newTestMemory(t)

	// verify memory size set by program is 17 pages
	memLen, err := mem.Len()
	require.NoError(err)
	require.Equal(uint32(17*MemoryPageSize), memLen)

	// grow memory by 1 page which is the default max memory (18 pages)
	// _, err = mem.Grow(1)
	// require.NoError(err)
	// memLen, err = mem.Len()
	// require.NoError(err)
	// require.Equal(uint32(engine.DefaultLimitMaxMemory), memLen)

	// allocate entire memory
	ptr, err := mem.Alloc(1)
	require.NoError(err)

	// write to memory
	bytes := make([]byte, 2)
	err = mem.Write(ptr, bytes)
	require.NoError(err)
}

func newTestMemory(t *testing.T) *Memory {
	require := require.New(t)
	wasmBytes := tests.ReadFixture(t, "../tests/fixture/memory.wasm")

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
	mem := NewMemory(wmem, allocFunc, store.Get())
	require.NotNil(mem)
	return mem
}
