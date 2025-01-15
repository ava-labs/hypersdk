// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
)

var _ Execution = (*ExecutionNoOp)(nil)

type ExecutionNoOp struct{}

func (*ExecutionNoOp) ImmutableView(_ context.Context, _ state.Keys, im state.Immutable, _ uint64) (state.Immutable, error) {
	return im, nil
}

func (*ExecutionNoOp) MutableView(mu state.Mutable, _ uint64) state.Mutable {
	return mu
}
