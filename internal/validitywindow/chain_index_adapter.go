// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/internal/emap"
)

type ChainIndexAdapter[T ExecutionBlock[U], U emap.Item] struct {
	GetBlock func(ctx context.Context, blkID ids.ID) (T, error)
}

func NewChainIndexAdapter[T ExecutionBlock[U], U emap.Item](
	getBlock func(ctx context.Context, blkID ids.ID) (T, error),
) *ChainIndexAdapter[T, U] {
	return &ChainIndexAdapter[T, U]{
		GetBlock: getBlock,
	}
}

func (c *ChainIndexAdapter[T, U]) GetExecutionBlock(ctx context.Context, blkID ids.ID) (ExecutionBlock[U], error) {
	block, err := c.GetBlock(ctx, blkID)
	if err != nil {
		return nil, err
	}
	return block, nil
}
