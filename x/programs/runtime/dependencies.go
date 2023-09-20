// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/bytecodealliance/wasmtime-go/v12"
)

type ProgramBytes []byte

type EngineCompileStrategy uint8

const (
	// CompileWasm will compile the wasm module before instantiating it.
	CompileWasm EngineCompileStrategy = iota
	// PrecompiledWasm accepts a precompiled wasm module serialized by an Engine.
	PrecompiledWasm
)

var NoImports = make(Imports)

type Link struct {
	*wasmtime.Linker
}

type Runtime interface {
	// Initialize initializes the runtime with the given program bytes. The engine will
	// handle the compile strategy and instantiate the module with the given imports.
	// Initialize should only be called once.
	Initialize(context.Context, ProgramBytes) error
	// Call invokes the an exported guest function with the given parameters.
	Call(context.Context, string, ...interface{}) ([]uint64, error)
	// Memory returns the runtime memory.
	Memory() Memory
	// Meter returns the runtime meter.
	Meter() Meter
	// Stop stops the runtime.
	Stop()
}

// TODO: abstract client interface so that the client doesn't need to be runtime specific/dependent.
type WasmtimeExportClient interface {
	// GetExportedFunction returns a function exported by the guest module.
	ExportedFunction(string) (*wasmtime.Func, error)
	// GetExportedMemory returns the memory exported by the guest module.
	GetMemory() (*wasmtime.Memory, error)
	// GetExportedTable returns the store exported by the guest module.
	Store() wasmtime.Storelike
}

type Imports map[string]Import

// Import defines host functions exposed by this runtime that can be imported by
// a guest module.
type Import interface {
	// Name returns the name of this import module.
	Name() string
	// Instantiate instantiates an all of the functions exposed by this import module.
	Register(Link, Meter) error
}

// Memory defines the interface for interacting with memory.
type Memory interface {
	// Range returns a slice of data from a specified offset.
	Range(uint32, uint32) ([]byte, error)
	// Alloc allocates a block of memory and returns a pointer
	// (offset) to its location on the stack.
	Alloc(uint32) (uint32, error)
	// Write writes the given data to the memory at the given offset.
	Write(uint32, []byte) error
	// Len returns the length of this memory in bytes.
	Len() (uint64, error)
	// Grow increases the size of the memory pages by delta.
	Grow(uint64) (uint64, error)
}

type Meter interface {
	// GetBalance returns the balance of the meters units remaining.
	GetBalance() uint64
	// Spend attempts to spend the given amount of units. If the meter has
	Spend(uint64) (uint64, error)
	// AddUnits add units back to the meters balance.
	AddUnits(uint64) error
}
