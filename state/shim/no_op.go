// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

type NoOp struct{}

func (*NoOp) MutableView(mu state.Mutable, _ uint64) state.Mutable {
	return mu
}

func (*NoOp) ImmutableView(im state.Immutable) state.Immutable {
	return im
}

func (*NoOp) RawState(*tstate.TState, uint64) error {
	return nil
}

func (*NoOp) ExecutableState(
	mp map[string][]byte,
	_ uint64,
) (state.Immutable, error) {
	return state.ImmutableStorage(mp), nil
}
