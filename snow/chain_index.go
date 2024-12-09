// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type ChainIndex[I Block, O Block, A Block] struct {
	*CovariantVM[I, O, A]
}

func (c *ChainIndex[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := c.CovariantVM.GetBlock(ctx, blkID)
	if err != nil {
		var emptyBlk I
		return emptyBlk, err
	}
	return blk.Input, nil
}

func (c *ChainIndex[I, O, A]) GetPreferredBlock(ctx context.Context) (O, error) {
	blk, err := c.CovariantVM.GetBlock(ctx, c.preferredBlkID)
	if err != nil {
		var emptyBlk O
		return emptyBlk, err
	}
	return blk.Output, nil
}

func (c *ChainIndex[I, O, A]) GetLastAccepted(ctx context.Context) A {
	return c.CovariantVM.lastAcceptedBlock.Accepted
}
