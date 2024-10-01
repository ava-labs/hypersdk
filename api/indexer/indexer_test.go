// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/fees"
)

func TestBlockIndex(t *testing.T) {
	require := require.New(t)

	tempDir := t.TempDir()
	blockWindow := 2
	indexer, err := NewIndexer(tempDir, chaintest.NewEmptyParser(), uint64(blockWindow))
	require.NoError(err)

	executedBlocks := make([]*chain.ExecutedBlock, blockWindow*2)
	parentID := ids.GenerateTestID()
	for i := 0; i < len(executedBlocks); i++ {
		statelessBlock := &chain.StatelessBlock{
			Prnt:   parentID,
			Tmstmp: 1 + int64(i),
			Hght:   1 + uint64(i),
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
		err = indexer.Accept(blk)
		require.NoError(err)
	}

	expectedLatestBlk := executedBlocks[len(executedBlocks)-1]
	receivedLatestBlk, err := indexer.GetLatestBlock()
	require.NoError(err)
	require.Equal(expectedLatestBlk.BlockID, receivedLatestBlk.BlockID)

	checkBlocks := func(indexer *Indexer, expectedBlocks []*chain.ExecutedBlock, blockWindow int) {
		for i := 0; i < blockWindow; i++ {
			expectedBlk := expectedBlocks[len(expectedBlocks)-1-i]
			height := expectedBlk.Block.Hght
			blkByHeight, err := indexer.GetBlockByHeight(height)
			require.NoError(err)
			require.Equal(expectedBlk.BlockID, blkByHeight.BlockID)

			blkByID, err := indexer.GetBlock(expectedBlk.BlockID)
			require.NoError(err)
			require.Equal(expectedBlk.BlockID, blkByID.BlockID)
		}

		for i := 0; i <= len(executedBlocks)-blockWindow; i++ {
			_, err := indexer.GetBlockByHeight(uint64(i))
			require.ErrorIs(err, errBlockNotFound)
		}
	}

	checkBlocks(indexer, executedBlocks, blockWindow)
	require.NoError(indexer.Close())

	restartedIndexer, err := NewIndexer(tempDir, chaintest.NewEmptyParser(), uint64(blockWindow))
	require.NoError(err)
	checkBlocks(indexer, executedBlocks, blockWindow)
	require.NoError(restartedIndexer.Close())

	restartedIndexerSingleBlockWindow, err := NewIndexer(tempDir, chaintest.NewEmptyParser(), 1)
	require.NoError(err)
	checkBlocks(restartedIndexerSingleBlockWindow, executedBlocks, 1)
	require.NoError(restartedIndexerSingleBlockWindow.Close())
}
