// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
)

func GenerateEmptyExecutedBlocks[T chain.Action[T], A chain.Auth[A]](
	require *require.Assertions,
	parentID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	timestampOffset int64,
	numBlocks int,
) []*chain.ExecutedBlock[T, A] {
	executedBlocks := make([]*chain.ExecutedBlock[T, A], numBlocks)
	for i := range executedBlocks {
		block := chain.NewBlock[T, A](
			parentID,
			parentTimestamp+timestampOffset*int64(i),
			parentHeight+1+uint64(i),
			[]*chain.Transaction[T, A]{},
			ids.Empty,
			nil,
		)
		parentID = block.GetID()

		blk := chain.NewExecutedBlock(
			block,
			&chain.ExecutionResults{},
		)
		executedBlocks[i] = blk
	}
	return executedBlocks
}
