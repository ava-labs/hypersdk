// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

// Execution is responsible for translating between raw state and execution state.
// This is useful for cases like managing suffix values
type Execution interface {
	// ExecutableState converts a kv-store to a state which is suitable for TStateView
	ExecutableState(map[string][]byte, uint64) (state.Immutable, error)
	// MutableView is used for genesis initialization
	MutableView(state.Mutable, uint64) state.Mutable
	// ImmutableView is used for VM transaction pre-execution
	ImmutableView(state.Immutable) state.Immutable
	// ExecutableState converts the state changes of a block into a format
	// suitable for DB storage
	RawState(*tstate.TState, uint64) error
}
