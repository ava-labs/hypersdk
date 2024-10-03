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
		statelessBlock := &chain.StatelessBlock{
			Prnt:   parentID,
			Tmstmp: parentTimestamp + timestampOffset*int64(i),
			Hght:   parentHeight + uint64(i),
			Txs:    []*chain.Transaction{},
		}
		blkID, err := statelessBlock.ID()
		require.NoError(err)
		parentID = blkID

		blk, err := chain.NewExecutedBlock(
			statelessBlock,
			[]*chain.Result{},
			fees.Dimensions{},
		)
		require.NoError(err)
		executedBlocks[i] = blk
		require.NoError(err)
	}
	return executedBlocks
}
