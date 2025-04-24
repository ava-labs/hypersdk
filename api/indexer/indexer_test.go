// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
)

func createTestIndexer(
	t *testing.T,
	ctx context.Context,
	numExecutedBlocks int,
	blockWindow int,
	numTxs int,
) (indexer *Indexer, executedBlocks []*chain.ExecutedBlock, indexerDir string) {
	r := require.New(t)

	tempDir := t.TempDir()
	indexer, err := NewIndexer(tempDir, chaintest.NewTestParser(), uint64(blockWindow))
	r.NoError(err)

	executedBlocks = chaintest.GenerateEmptyExecutedBlocks(
		r,
		ids.GenerateTestID(),
		ids.GenerateTestID(),
		0,
		0,
		1,
		numExecutedBlocks,
		numTxs,
	)
	for _, blk := range executedBlocks {
		err = indexer.Notify(ctx, blk)
		r.NoError(err)
	}
	return indexer, executedBlocks, tempDir
}

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
	require.Equal(expectedLatestBlk.Block.GetID(), latestBlk.Block.GetID())

	// Confirm all blocks in the window are retrievable
	for i := 0; i < blockWindow; i++ {
		expectedBlk := expectedBlocks[len(expectedBlocks)-1-i]
		height := expectedBlk.Block.Hght
		blkByHeight, err := indexer.GetBlockByHeight(height)
		require.NoError(err)
		require.Equal(expectedBlk.Block.GetID(), blkByHeight.Block.GetID())

		blkByID, err := indexer.GetBlock(expectedBlk.Block.GetID())
		require.NoError(err)
		require.Equal(expectedBlk.Block.GetID(), blkByID.Block.GetID())

		// confirm all transactions are available
		for txIndex, expectedTx := range expectedBlk.Block.Txs {
			txID := expectedTx.GetID()
			found, actualTx, _, res, err := indexer.GetTransaction(txID)
			require.NoError(err)
			require.True(found)
			require.Equal(expectedTx, actualTx)
			require.Equal(expectedBlk.ExecutionResults.Results[txIndex], res)
		}
	}

	// Confirm blocks outside the window are not retrievable
	for i := 0; i <= len(expectedBlocks)-blockWindow; i++ {
		_, err := indexer.GetBlockByHeight(uint64(i))
		require.ErrorIs(err, errBlockNotFound, "height=%d", i)

		// Confirm all transactions outside of the window are notretrievable
		expectedBlk := expectedBlocks[i]
		for _, tx := range expectedBlk.Block.Txs {
			txID := tx.GetID()
			found, _, _, _, err := indexer.GetTransaction(txID)
			require.NoError(err)
			require.False(found)
		}
	}
}

func TestBlockIndex(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	var (
		numExecutedBlocks = 4
		blockWindow       = 2
		numTxs            = 0
	)
	indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)
	// Confirm we have indexed the expected window of blocks
	checkBlocks(r, indexer, executedBlocks, blockWindow)
	r.NoError(indexer.Close())
}

func TestBlockIndexRestart(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	var (
		numExecutedBlocks = 4
		blockWindow       = 2
		numTxs            = 0
	)
	indexer, executedBlocks, indexerDir := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)

	// Confirm we have indexed the expected window of blocks
	checkBlocks(r, indexer, executedBlocks, blockWindow)
	r.NoError(indexer.Close())

	// Confirm we have indexed the expected window of blocks after restart
	restartedIndexer, err := NewIndexer(indexerDir, chaintest.NewTestParser(), uint64(blockWindow))
	r.NoError(err)
	checkBlocks(r, indexer, executedBlocks, blockWindow)
	r.NoError(restartedIndexer.Close())

	// Confirm we have indexed the expected window of blocks after restart and a window
	// change
	restartedIndexerSingleBlockWindow, err := NewIndexer(indexerDir, chaintest.NewTestParser(), 1)
	r.NoError(err)
	checkBlocks(r, restartedIndexerSingleBlockWindow, executedBlocks, 1)
	r.NoError(restartedIndexerSingleBlockWindow.Close())
}

func TestInvalidBlockWindowSizes(t *testing.T) {
	r := require.New(t)
	blockWindow := uint64(0)
	_, err := NewIndexer(t.TempDir(), chaintest.NewTestParser(), blockWindow)
	r.ErrorIs(err, errZeroBlockWindow)

	blockWindow = maxBlockWindow + 1
	_, err = NewIndexer(t.TempDir(), chaintest.NewTestParser(), blockWindow)
	r.ErrorIs(err, errInvalidBlockWindowSize)
}
