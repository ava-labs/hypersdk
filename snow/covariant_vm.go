// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/hypersdk/utils"
)

type CovariantVM[I Block, O Block, A Block] struct {
	*VM[I, O, A]
}

func (v *VM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (snowman.Block, error) {
	return v.covariantVM.GetBlock(ctx, blkID)
}

func (v *CovariantVM[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.GetBlock")
	defer span.End()

	// Check verified map
	v.verifiedL.RLock()
	if blk, exists := v.verifiedBlocks[blkID]; exists {
		v.verifiedL.RUnlock()
		return blk, nil
	}
	v.verifiedL.RUnlock()

	// Check if last accepted block or recently accepted
	if v.lastAcceptedBlock.ID() == blkID {
		return v.lastAcceptedBlock, nil
	}
	if blk, ok := v.acceptedBlocksByID.Get(blkID); ok {
		return blk, nil
	}
	// Retrieve and parse from disk
	// Note: this returns an accepted block with only the input block set.
	// The consensus engine guarantees that:
	// 1. Verify is only called on a block whose parent is lastAcceptedBlock or in verifiedBlocks
	// 2. Accept is only called on a block whose parent is lastAcceptedBlock
	blk, err := v.chain.GetBlock(ctx, blkID)
	if err != nil {
		return nil, err
	}
	return v.ParseBlock(ctx, blk)
}

func (v *CovariantVM[I, O, A]) GetBlockByHeight(ctx context.Context, height uint64) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.GetBlockByHeight")
	defer span.End()

	if v.lastAcceptedBlock.Height() == height {
		return v.lastAcceptedBlock, nil
	}
	var blkID ids.ID
	if fetchedBlkID, ok := v.acceptedBlocksByHeight.Get(height); ok {
		blkID = fetchedBlkID
	} else {
		fetchedBlkID, err := v.chain.GetBlockIDAtHeight(ctx, height)
		if err != nil {
			return nil, err
		}
		blkID = fetchedBlkID
	}

	if blk, ok := v.acceptedBlocksByID.Get(blkID); ok {
		return blk, nil
	}

	return v.GetBlock(ctx, blkID)
}

func (v *VM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (snowman.Block, error) {
	return v.covariantVM.ParseBlock(ctx, bytes)
}

func (v *CovariantVM[I, O, A]) ParseBlock(ctx context.Context, bytes []byte) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.ParseBlock")
	defer span.End()

	blkID := utils.ToID(bytes)
	if existingBlk, err := v.GetBlock(ctx, blkID); err == nil {
		return existingBlk, nil
	}
	if blk, ok := v.parsedBlocks.Get(blkID); ok {
		return blk, nil
	}
	inputBlk, err := v.chain.ParseBlock(ctx, bytes)
	if err != nil {
		return nil, err
	}
	blk := NewInputBlock[I, O, A](v, inputBlk)
	v.parsedBlocks.Put(blkID, blk)
	return blk, nil
}

func (v *VM[I, O, A]) BuildBlock(ctx context.Context) (snowman.Block, error) {
	return v.covariantVM.BuildBlock(ctx)
}

func (v *CovariantVM[I, O, A]) BuildBlock(ctx context.Context) (*StatefulBlock[I, O, A], error) {
	ctx, span := v.tracer.Start(ctx, "VM.BuildBlock")
	defer span.End()

	preferredBlk, err := v.GetBlock(ctx, v.preferredBlkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get preferred block: %w", err)
	}
	inputBlock, outputBlock, err := v.chain.BuildBlock(ctx, preferredBlk.Output)
	if err != nil {
		return nil, err
	}
	sb := NewVerifiedBlock[I, O, A](v, inputBlock, outputBlock)
	sb.Output = outputBlock
	v.parsedBlocks.Put(sb.ID(), sb)

	return sb, nil
}

func (v *VM[I, O, A]) SetPreference(ctx context.Context, blkID ids.ID) error {
	v.preferredBlkID = blkID
	return nil
}

func (v *VM[I, O, A]) LastAccepted(context.Context) (ids.ID, error) {
	return v.lastAcceptedBlock.ID(), nil
}

func (v *CovariantVM[I, O, A]) LastAcceptedBlock(ctx context.Context) *StatefulBlock[I, O, A] {
	return v.lastAcceptedBlock
}
