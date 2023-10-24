// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

var _ WasmtimeExportClient = (*callerClient)(nil)

type callerClient struct {
	mod *wasmtime.Caller
}

// newExportClient returns a new export client for local use.
func newExportClient(inst *wasmtime.Instance, store *wasmtime.Store) WasmtimeExportClient {
	return &exportClient{inst: inst, store: store}
}

// NewExportClientFromCaller returns a new export client given a wasmtime caller module.
// This is useful for creating export modules.
func NewExportClient(mod *wasmtime.Caller) WasmtimeExportClient {
	return &callerClient{mod: mod}
}

func (c *callerClient) ExportedFunction(name string) (*wasmtime.Func, error) {
	ext := c.mod.GetExport(name)
	fn := ext.Func()
	if ext == nil || fn == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, name)
	}
	return fn, nil
}

func (c *callerClient) GetMemory() (*wasmtime.Memory, error) {
	ext := c.mod.GetExport(MemoryFnName)
	if ext == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, MemoryFnName)
	}
	memory := ext.Memory()
	if memory == nil {
		return nil, ErrMissingInvalidMemoryFunction
	}
	return memory, nil
}

func (c *callerClient) Store() wasmtime.Storelike {
	return c.mod
}

type exportClient struct {
	inst  *wasmtime.Instance
	store *wasmtime.Store
}

func (c *exportClient) ExportedFunction(name string) (*wasmtime.Func, error) {
	ext := c.inst.GetExport(c.store, name)
	fn := ext.Func()
	if ext == nil || fn == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, name)
	}
	return fn, nil
}

func (c *exportClient) GetMemory() (*wasmtime.Memory, error) {
	ext := c.inst.GetExport(c.store, MemoryFnName)
	if ext == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, MemoryFnName)
	}
	memory := ext.Memory()
	if memory == nil {
		return nil, ErrMissingInvalidMemoryFunction
	}
	return memory, nil
}

func (c *exportClient) Store() wasmtime.Storelike {
	return c.store
}
