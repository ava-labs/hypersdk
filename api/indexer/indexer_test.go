// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
)

// checkBlocks confirms that all blocks are retrievable/not retrievable as expected
func checkBlocks(
	require *require.Assertions,
	indexer *Indexer,
	expectedBlocks []*chain.ExecutedBlock,
	blockWindow int,
) {
	// Confirm we retrieve the correct latest block
	expectedLatestBlk := expectedBlocks[len(expectedBlocks)-1]
	latestBlk, err := indexer.GetLatestBlock()
	require.NoError(err)
	require.Equal(expectedLatestBlk.BlockID, latestBlk.BlockID)

	// Confirm all blocks in the window are retrievable
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

	// Confirm blocks outside the window are not retrievable
	for i := 0; i <= len(expectedBlocks)-blockWindow; i++ {
		_, err := indexer.GetBlockByHeight(uint64(i))
		require.ErrorIs(err, errBlockNotFound)
	}
}

func TestBlockIndex(t *testing.T) {
	require := require.New(t)

	tempDir := t.TempDir()
	blockWindow := 2
	indexer, err := NewIndexer(tempDir, chaintest.NewEmptyParser(), uint64(blockWindow))
	require.NoError(err)

	executedBlocks := chaintest.GenerateEmptyExecutedBlocks(
		require,
		ids.GenerateTestID(),
		0,
		0,
		1,
		blockWindow*2,
	)
	for _, blk := range executedBlocks {
		err = indexer.Accept(blk)
		require.NoError(err)
	}

	// Confirm we have indexed the expected window of blocks
	checkBlocks(require, indexer, executedBlocks, blockWindow)
	require.NoError(indexer.Close())

	// Confirm we have indexed the expected window of blocks after restart
	restartedIndexer, err := NewIndexer(tempDir, chaintest.NewEmptyParser(), uint64(blockWindow))
	require.NoError(err)
	checkBlocks(require, indexer, executedBlocks, blockWindow)
	require.NoError(restartedIndexer.Close())

	// Confirm we have indexed the expected window of blocks after restart and a window
	// change
	restartedIndexerSingleBlockWindow, err := NewIndexer(tempDir, chaintest.NewEmptyParser(), 1)
	require.NoError(err)
	checkBlocks(require, restartedIndexerSingleBlockWindow, executedBlocks, 1)
	require.NoError(restartedIndexerSingleBlockWindow.Close())
}
