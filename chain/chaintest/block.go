// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
)

func GenerateEmptyExecutedBlocks(
	require *require.Assertions,
	parentID ids.ID,
	parentHeight uint64,
	parentTimestamp int64,
	timestampOffset int64,
	numBlocks int,
) []*chain.ExecutedBlock {
	executedBlocks := make([]*chain.ExecutedBlock, numBlocks)
	for i := range executedBlocks {
		statelessBlock, err := chain.NewStatelessBlock(
			parentID,
			parentTimestamp+timestampOffset*int64(i),
			parentHeight+1+uint64(i),
			[]*chain.Transaction{},
			ids.Empty,
			nil,
		)
		require.NoError(err)
		parentID = statelessBlock.GetID()

		blk := chain.NewExecutedBlock(
			statelessBlock,
			[]*chain.Result{},
			fees.Dimensions{},
			fees.Dimensions{},
		)
		executedBlocks[i] = blk
	}
	return executedBlocks
}
