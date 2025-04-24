// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

// InputCovariantVM provides basic VM functionality that only returns the input block type
// Input is the only field guaranteed to be set for each consensus block, so we provide
// a wrapper that only exposes the input block type.
type InputCovariantVM[I Block, O Block, A Block] struct {
	vm *VM[I, O, A]
}

// GetBlock retrieves a block by ID and extracts its Input component.
// It calls the underlying VM.GetBlock and transforms StatefulBlock into just the Input block type.
// On error, returns a zero value of type I and the error.
func (v *InputCovariantVM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := v.vm.GetBlock(ctx, blkID)
	if err != nil {
		var emptyI I
		return emptyI, err
	}
	return blk.Input, nil
}

// GetBlockByHeight retrieves a block by height and extracts its Input component.
// It calls the underlying VM.GetBlockByHeight and transforms the StatefulBlock into just the Input block type.
// On error, returns a zero value of type I and the error.
func (v *InputCovariantVM[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (I, error) {
	blk, err := v.vm.GetBlockByHeight(ctx, height)
	if err != nil {
		var emptyI I
		return emptyI, err
	}
	return blk.Input, nil
}

// ParseBlock parses a block from bytes and extracts its Input component.
// It calls the underlying VM.ParseBlock and transforms the StatefulBlock into just the Input block type.
// On error, returns a zero value of type I and the error.
func (v *InputCovariantVM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (I, error) {
	blk, err := v.vm.ParseBlock(ctx, bytes)
	if err != nil {
		var emptyI I
		return emptyI, err
	}
	return blk.Input, nil
}

// LastAcceptedBlock returns the Input component of the last accepted block.
// It extracts the Input field from the StatefulBlock returned by VM.LastAcceptedBlock.
func (v *InputCovariantVM[I, O, A]) LastAcceptedBlock(ctx context.Context) I {
	return v.vm.LastAcceptedBlock(ctx).Input
}
