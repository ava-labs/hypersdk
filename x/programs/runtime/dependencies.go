// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/bytecodealliance/wasmtime-go/v14"
)

type Runtime interface {
	// Initialize initializes the runtime with the given program bytes and max
	// units. The engine will handle the compile strategy and instantiate the
	// module with the given imports.  Initialize should only be called once.
	Initialize(context.Context, []byte, uint64) error
	// Call invokes an exported guest function with the given parameters.
	// Returns the results of the call or an error if the call failed.
	// If the function called does not return a result this value is set to nil.
	Call(context.Context, string, ...SmartPtr) ([]int64, error)
	// Memory returns the runtime memory.
	Memory() Memory
	// Meter returns the runtime meter.
	Meter() *engine.Meter
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

// Memory defines the interface for interacting with memory.
type Memory interface {
	// Range returns an owned slice of data from a specified offset.
	Range(uint64, uint64) ([]byte, error)
	// Alloc allocates a block of memory and returns a pointer
	// (offset) to its location on the stack.
	Alloc(uint64) (uint64, error)
	// Write writes the given data to the memory at the given offset.
	Write(uint64, []byte) error
	// Len returns the length of this memory in bytes.
	Len() (uint64, error)
	// Grow increases the size of the memory pages by delta.
	Grow(uint64) (uint64, error)
}
