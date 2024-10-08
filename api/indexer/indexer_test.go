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

func TestBlockIndex(t *testing.T) {
	require := require.New(t)
	var (
		numExecutedBlocks = 4
	)
	indexer, executedBlocks, _ := createTestIndexer(t, numExecutedBlocks)
	// Confirm we have indexed the expected window of blocks
	checkBlocks(t, indexer, executedBlocks)
	require.NoError(indexer.Close())
}

func TestBlockIndexRestart(t *testing.T) {
	require := require.New(t)
	var (
		numExecutedBlocks = 4
	)
	indexer, executedBlocks, indexerDir := createTestIndexer(t, numExecutedBlocks)

	// Confirm we have indexed the expected window of blocks
	checkBlocks(t, indexer, executedBlocks)
	require.NoError(indexer.Close())

	// Confirm we have indexed the expected window of blocks after restart
	restartedIndexer, err := NewIndexer(indexerDir, chaintest.NewEmptyParser())
	require.NoError(err)
	checkBlocks(t, restartedIndexer, executedBlocks)
	require.NoError(restartedIndexer.Close())

	// Confirm we have indexed the expected window of blocks after restart
	restartedIndexerSingleBlockWindow, err := NewIndexer(indexerDir, chaintest.NewEmptyParser())
	require.NoError(err)
	checkBlocks(t, restartedIndexerSingleBlockWindow, executedBlocks)
	require.NoError(restartedIndexerSingleBlockWindow.Close())
}

func createTestIndexer(
	t *testing.T,
	numExecutedBlocks int,
) (indexer *Indexer, executedBlocks []*chain.ExecutedBlock, indexerDir string) {
	require := require.New(t)

	tempDir := t.TempDir()
	indexer, err := NewIndexer(tempDir, chaintest.NewEmptyParser())
	require.NoError(err)

	executedBlocks = chaintest.GenerateEmptyExecutedBlocks(
		require,
		ids.GenerateTestID(),
		0,
		0,
		1,
		numExecutedBlocks,
	)
	for _, blk := range executedBlocks {
		err = indexer.Accept(blk)
		require.NoError(err)
	}
	return indexer, executedBlocks, tempDir
}

// checkBlocks confirms that all blocks are retrievable/not retrievable as expected
func checkBlocks(
	t *testing.T,
	indexer *Indexer,
	expectedBlocks []*chain.ExecutedBlock,
) {
	t.Helper()
	require := require.New(t)

	// Confirm we retrieve the correct latest block
	expectedLatestBlk := expectedBlocks[len(expectedBlocks)-1]
	latestBlk, err := indexer.GetLatestBlock()
	require.NoError(err)
	require.Equal(expectedLatestBlk.BlockID, latestBlk.BlockID)

	// Confirm all blocks are retrievable
	for _, expectedBlk := range expectedBlocks {
		blkByHeight, err := indexer.GetBlockByHeight(expectedBlk.Block.Hght)
		require.NoError(err)
		require.Equal(expectedBlk.BlockID, blkByHeight.BlockID)

		blkByID, err := indexer.GetBlock(expectedBlk.BlockID)
		require.NoError(err)
		require.Equal(expectedBlk.BlockID, blkByID.BlockID)
	}
}
