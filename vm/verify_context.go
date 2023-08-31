// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ chain.VerifyContext = (*AcceptedVerifyContext)(nil)
	_ chain.VerifyContext = (*PendingVerifyContext)(nil)
)

func (vm *VM) GetVerifyContext(ctx context.Context, blockHeight uint64, parent ids.ID) (chain.VerifyContext, error) {
	// If [blockHeight] is 0, we throw an error because there is no pre-genesis verification context.
	if blockHeight == 0 {
		return nil, errors.New("cannot get context of genesis block")
	}

	// If the last accepted block is not yet processed, we can't use the accepted state for the
	// verification context. This could happen if state sync finishes with no processing blocks (we
	// sync to the post-execution state of the parent of the last accepted block, not the post-execution
	// state of the last accepted block).
	//
	// Invariant: When [View] is called on [vm.lastAccepted], the block will be verified and the accepted
	// state will be updated.
	if !vm.lastAccepted.Processed() && parent == vm.lastAccepted.ID() {
		return &PendingVerifyContext{vm.lastAccepted}, nil
	}

	// If the parent block is not yet accepted, we should return the block's processing parent (it may
	// or may not be verified yet).
	if blockHeight-1 > vm.lastAccepted.Hght {
		blk, err := vm.GetStatelessBlock(ctx, parent)
		if err != nil {
			return nil, err
		}
		return &PendingVerifyContext{blk}, nil
	}

	// If the parent block is accepted and processed, we should
	// just use the accepted state as the verification context.
	return &AcceptedVerifyContext{vm}, nil
}

type PendingVerifyContext struct {
	blk *chain.StatelessBlock
}

func (p *PendingVerifyContext) View(ctx context.Context, blockRoot *ids.ID, verify bool) (state.View, error) {
	return p.blk.View(ctx, blockRoot, verify)
}

func (p *PendingVerifyContext) IsRepeat(ctx context.Context, oldestAllowed int64, txs []*chain.Transaction, marker set.Bits, stop bool) (set.Bits, error) {
	return p.blk.IsRepeat(ctx, oldestAllowed, txs, marker, stop)
}

type AcceptedVerifyContext struct {
	vm *VM
}

// We disregard [verify] because [GetVerifyContext] ensures
// we will never need to verify a block if [AcceptedVerifyContext] is returned.
func (a *AcceptedVerifyContext) View(ctx context.Context, blockRoot *ids.ID, _ bool) (state.View, error) {
	state, err := a.vm.State()
	if err != nil {
		return nil, err
	}
	if blockRoot != nil {
		// This does not make deferred root generation less
		// efficient because the root must have already
		// been calculated before the latest state was written
		// to disk.
		root, err := state.GetMerkleRoot(ctx)
		if err != nil {
			return nil, err
		}
		if root != *blockRoot {
			// This should never happen but we check
			// this to check subtle state handling bugs
			// in the [chain] package.
			return nil, ErrUnexpectedStateRoot
		}
	}
	return state, nil
}

func (a *AcceptedVerifyContext) IsRepeat(ctx context.Context, _ int64, txs []*chain.Transaction, marker set.Bits, stop bool) (set.Bits, error) {
	bits := a.vm.IsRepeat(ctx, txs, marker, stop)
	return bits, nil
}
