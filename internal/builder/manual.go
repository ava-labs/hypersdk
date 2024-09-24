// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/common"

	"github.com/ava-labs/hypersdk/chain"
)

var _ Builder = (*Manual[chain.PendingView])(nil)

type Manual[T chain.PendingView] struct {
	vm        VM[T]
	doneBuild chan struct{}
}

func NewManual[T chain.PendingView](vm VM[T]) *Manual[T] {
	return &Manual[T]{
		vm:        vm,
		doneBuild: make(chan struct{}),
	}
}

func (b *Manual[_]) Run() {
	close(b.doneBuild)
}

// Queue is a no-op in [Manual].
func (*Manual[_]) Queue(context.Context) {}

func (b *Manual[_]) Force(context.Context) error {
	select {
	case b.vm.EngineChan() <- common.PendingTxs:
	default:
		b.vm.Logger().Debug("dropping message to consensus engine")
	}
	return nil
}

func (b *Manual[_]) Done() {
	<-b.doneBuild
}
