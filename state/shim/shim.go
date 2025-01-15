// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
)

type Execution interface {
	ImmutableView(context.Context, state.Keys, state.Immutable, uint64) (state.Immutable, error)
	// TODO: we should be able to get of this if we modify the genesis logic
	MutableView(state.Mutable, uint64) state.Mutable
}
