// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
)

func (c *Chain) AssembleBlock(
	ctx context.Context,
	parentView state.View,
	parent *ExecutionBlock,
	timestamp int64,
	blockHeight uint64,
	txs []*Transaction,
) (*ExecutionBlock, error) {
	ctx, span := c.tracer.Start(ctx, "chain.AssembleBlock")
	defer span.End()

	parentStateRoot, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}

	sb, err := NewStatelessBlock(
		parent.ID(),
		timestamp,
		blockHeight,
		txs,
		parentStateRoot,
	)
	if err != nil {
		return nil, err
	}
	return NewExecutionBlock(ctx, sb, c, true)
}

func (c *Chain) ExecuteBlock(
	ctx context.Context,
	parentView state.View,
	b *ExecutionBlock,
) (*ExecutedBlock, state.View, error) {
	ctx, span := c.tracer.Start(ctx, "chain.ExecuteBlock")
	defer span.End()

	return c.Execute(ctx, parentView, b)
}
