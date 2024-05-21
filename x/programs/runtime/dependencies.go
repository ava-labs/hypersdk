// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

type Runtime interface {
	// Initialize initializes the runtime with the given program bytes and max
	// units. The engine will handle the compile strategy and instantiate the
	// module with the given imports.  Initialize should only be called once.
	Initialize(context.Context, *program.Context, []byte, uint64) error
	// Call invokes an exported guest function with the given parameters.
	// Returns the results of the call or an error if the call failed.
	// If the function called does not return a result this value is set to nil.
	Call(context.Context, string, *program.Context, ...uint32) ([]byte, error)
	// Memory returns the program memory.
	Memory() (*program.Memory, error)
	// Meter returns the engine meter.
	Meter() *engine.Meter
	// Stop stops the runtime.
	Stop()
}
