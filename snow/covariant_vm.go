// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"
	"fmt"
	"slices"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/utils"
)

type CovariantVM[I Block, O Block, A Block] struct {
	*VM[I, O, A]
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
	v.parsedBlocks.Put(sb.ID(), sb)

	return sb, nil
}

// getExclusiveBlockRange returns the exclusive range of blocks (startBlock, endBlock)
func (v *CovariantVM[I, O, A]) getExclusiveBlockRange(ctx context.Context, startBlock *StatefulBlock[I, O, A], endBlock *StatefulBlock[I, O, A]) ([]*StatefulBlock[I, O, A], error) {
	if startBlock.ID() == endBlock.ID() {
		return nil, nil
	}

	diff, err := math.Sub(endBlock.Height(), startBlock.Height())
	if err != nil {
		return nil, fmt.Errorf("failed to calculate height difference for exclusive block range: %w", err)
	}
	if diff == 0 {
		return nil, fmt.Errorf("cannot fetch invalid block range (%s, %s)", startBlock, endBlock)
	}
	blkRange := make([]*StatefulBlock[I, O, A], 0, diff)
	blk := endBlock
	for {
		blk, err = v.GetBlock(ctx, blk.Parent())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch parent of %s while fetching exclusive block range (%s, %s): %w", blk, startBlock, endBlock, err)
		}
		if blk.ID() == startBlock.ID() {
			break
		}
		if blk.Height() <= startBlock.Height() {
			return nil, fmt.Errorf("invalid block range (%s, %s) terminated at %s", startBlock, endBlock, blk)
		}
		blkRange = append(blkRange, blk)
	}
	slices.Reverse(blkRange)
	return blkRange, nil
}

func (v *CovariantVM[I, O, A]) LastAcceptedBlock(ctx context.Context) *StatefulBlock[I, O, A] {
	return v.lastAcceptedBlock
}
