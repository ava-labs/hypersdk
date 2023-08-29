// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
)

type Runtime interface {
	// Create initializes and calls the init function.
	Create(context.Context, []byte) (uint64, error)
	// Initialize's the wasm engine.
	Initialize(context.Context, []byte) error
	// Call invokes the function with the given parameters and returns any results
	// or an error for any failure looking up or invoking the function.
	Call(context.Context, string, ...uint64) ([]uint64, error)
	// GetGuestBuffer returns a buffer from the guest at [offset] with length [length]. Returns
	// false if out of range.
	GetGuestBuffer(uint32, uint32) ([]byte, bool)
	// WriteGuestBuffer allocates buf to the heap on the guest and returns the offset.
	WriteGuestBuffer(context.Context, []byte) (uint64, error)
	// Stop performs a shutdown of the engine.
	Stop(context.Context) error
}

type Storage interface {
	Get(context.Context, uint32) ([]byte, bool, error)
	Set(context.Context, uint32, uint32, []byte) error
}
