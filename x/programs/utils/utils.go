// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero/api"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/state"
)

type testDB struct {
	db *memdb.Database
}

func NewTestDB() state.Mutable {
	return &testDB{
		db: memdb.New(),
	}
}

func (c *testDB) GetValue(_ context.Context, key []byte) ([]byte, error) {
	return c.db.Get(key)
}

func (c *testDB) Insert(_ context.Context, key []byte, value []byte) error {
	return c.db.Put(key, value)
}

func (c *testDB) Remove(_ context.Context, key []byte) error {
	return c.db.Delete(key)
}

func GetGuestFnName(name string) string {
	return name + "_guest"
}

// WriteBuffer allocates [buffer] in the module's memory and returns the pointer
// to the buffer or an error if one occurred. The module must have an exported
// function named "alloc".
func WriteBuffer(ctx context.Context, mod api.Module, buffer []byte) (uint64, error) {
	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return 0, fmt.Errorf("module doesn't have an exported function named 'alloc'")
	}
	result, err := alloc.Call(ctx, uint64(len(buffer)))
	if err != nil {
		return 0, err
	}

	ptr := result[0]
	// write to the pointer WASM returned
	if !mod.Memory().Write(uint32(ptr), buffer) {
		return 0, fmt.Errorf("memory.Write(%d, %d) out of range of memory size %d",
			ptr, len(buffer), mod.Memory().Size())
	}

	return ptr, nil
}

// GetBuffer returns the buffer at [ptr] with length [length]. Returns
// false if out of range.
func GetBuffer(mod api.Module, ptr uint32, length uint32) ([]byte, bool) {
	return mod.Memory().Read(ptr, length)
}

func GetProgramBytes(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}
