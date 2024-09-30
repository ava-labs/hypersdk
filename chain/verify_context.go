// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/state"
)

var (
	_ VerifyContext = (*AcceptedVerifyContext)(nil)
	_ VerifyContext = (*PendingVerifyContext)(nil)
)

type PendingVerifyContext struct {
	blk *StatefulBlock
}

func (p *PendingVerifyContext) View(ctx context.Context, verify bool) (state.View, error) {
	return p.blk.View(ctx, verify)
}

func (p *PendingVerifyContext) IsRepeat(ctx context.Context, oldestAllowed int64, txs []*Transaction, marker set.Bits, stop bool) (set.Bits, error) {
	return p.blk.IsRepeat(ctx, oldestAllowed, txs, marker, stop)
}

type AcceptedVerifyContext struct {
	vm VM
}

// We disregard [verify] because [GetVerifyContext] ensures
// we will never need to verify a block if [AcceptedVerifyContext] is returned.
func (a *AcceptedVerifyContext) View(context.Context, bool) (state.View, error) {
	return a.vm.State()
}

func (a *AcceptedVerifyContext) IsRepeat(ctx context.Context, _ int64, txs []*Transaction, marker set.Bits, stop bool) (set.Bits, error) {
	bits := a.vm.IsRepeat(ctx, txs, marker, stop)
	return bits, nil
}
