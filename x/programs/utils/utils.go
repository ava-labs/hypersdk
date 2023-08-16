package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/tetratelabs/wazero/api"
)

type testDB struct {
	db *memdb.Database
}

func NewTestDB() chain.Database {
	return &testDB{
		db: memdb.New(),
	}
}

func (c *testDB) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	return c.db.Get(key)
}

func (c *testDB) Insert(ctx context.Context, key []byte, value []byte) error {
	return c.db.Put(key, value)
}

func (c *testDB) Remove(ctx context.Context, key []byte) error {
	return c.db.Delete(key)
}

// Move to utils package
func GetGuestFnName(name string) string {
	return name + "_guest"
}

// WriteBuffer allocts [buffer] in the module's memory and returns the pointer to the
// buffer or an error if one occured. The module must have an exported function named "alloc".
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
	// https://github.com/tetratelabs/wazero/blob/451a1b63a0554a2615cccb4bb424c6e6974105f6/examples/allocation/rust/greet.go#L69-L73
	if !mod.Memory().Write(uint32(ptr), buffer[:]) {
		return 0, fmt.Errorf("memory.Write(%d, %d) out of range of memory size %d",
			ptr, len(buffer), mod.Memory().Size())
	}

	return ptr, nil
}

// GetBuffer returns the buffer at [ptr] with length [length]. Returns
// false if out of range.
func GetBuffer(mod api.Module, ptr uint32, length uint32) ([]byte, bool) {
	buf, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return nil, false
	}
	return buf, true
}

func GetProgramBytes(filePath string) ([]byte, error) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return []byte(fileContent), nil
}