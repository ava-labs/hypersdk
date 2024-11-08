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
) (*ExecutedBlock, state.View, error) {
	ctx, span := c.tracer.Start(ctx, "chain.AssembleBlock")
	defer span.End()

	parentStateRoot, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, nil, err
	}

	sb, err := NewStatelessBlock(
		parent.ID(),
		timestamp,
		blockHeight,
		txs,
		parentStateRoot,
	)
	if err != nil {
		return nil, nil, err
	}
	executionBlock := NewExecutionBlock(sb)
	return c.Execute(ctx, parentView, executionBlock)
}
