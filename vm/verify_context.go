// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
)

var (
	_ VerifyContext = (*AcceptedVerifyContext)(nil)
	_ VerifyContext = (*PendingVerifyContext)(nil)
)

type VerifyContext interface {
	View(ctx context.Context, verify bool) (state.View, error)
}

type PendingVerifyContext struct {
	blk *StatefulBlock
}

func (p *PendingVerifyContext) View(ctx context.Context, verify bool) (state.View, error) {
	return p.blk.View(ctx, verify)
}

type AcceptedVerifyContext struct {
	vm *VM
}

// We disregard [verify] because [GetVerifyContext] ensures
// we will never need to verify a block if [AcceptedVerifyContext] is returned.
func (a *AcceptedVerifyContext) View(context.Context, bool) (state.View, error) {
	return a.vm.State()
}
