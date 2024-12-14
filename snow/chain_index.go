// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type BlockChainIndex[T Block] interface {
	Accept(ctx context.Context, blk T) error
	GetLastAcceptedHeight(ctx context.Context) (uint64, error)
	GetBlock(ctx context.Context, blkID ids.ID) (T, error)
	GetBlockIDAtHeight(ctx context.Context, blkHeight uint64) (ids.ID, error)
	GetBlockIDHeight(_ context.Context, blkID ids.ID) (uint64, error)
	GetBlockByHeight(ctx context.Context, blkHeight uint64) (T, error)
}

type ChainIndex[I Block, O Block, A Block] struct {
	covariantVM *CovariantVM[I, O, A]
}

func (c *ChainIndex[I, O, A]) GetBlock(ctx context.Context, blkID ids.ID) (I, error) {
	blk, err := c.covariantVM.GetBlock(ctx, blkID)
	if err != nil {
		var emptyBlk I
		return emptyBlk, err
	}
	return blk.Input, nil
}

func (c *ChainIndex[I, O, A]) GetPreferredBlock(ctx context.Context) (O, error) {
	blk, err := c.covariantVM.GetBlock(ctx, c.covariantVM.preferredBlkID)
	if err != nil {
		var emptyBlk O
		return emptyBlk, err
	}
	return blk.Output, nil
}

func (c *ChainIndex[I, O, A]) GetLastAccepted(ctx context.Context) A {
	return c.covariantVM.lastAcceptedBlock.Accepted
}
