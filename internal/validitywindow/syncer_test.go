// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

type testCases struct {
	name            string
	validityWindow  int64
	numOfBlocks     int
	setupChainIndex func([]ExecutionBlock[container]) *testChainIndex
	setupFetcher    func([]ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]]
	verify          func(context.Context, *require.Assertions, []ExecutionBlock[container], *Syncer[container, ExecutionBlock[container]], *testChainIndex)
}

func TestSyncer_Start(t *testing.T) {
	tests := []testCases{
		{
			name:            "should return full validity window from cache",
			numOfBlocks:     15,
			validityWindow:  5,
			setupChainIndex: newTestChainIndex,
			setupFetcher: func(_ []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
				// no need for fetcher
				return nil
			},
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], _ *testChainIndex) {
				target := blkChain[len(blkChain)-1]
				err := syncer.Start(ctx, target)
				r.NoError(err)
				r.Equal(blkChain[len(blkChain)-1].GetHeight(), syncer.timeValidityWindow.lastAcceptedBlockHeight)
				// We're expecting oldestBlock to have height 8 because:
				// - We have 15 blocks (height 0-14)
				// - Validity window is 5 time units
				// - Given target block at height 14 (timestamp 14)
				// - We need blocks until timestamp difference > 5
				// - This happens at block height 8 (14 - 8 > 5)
				r.Equal(blkChain[8].GetHeight(), syncer.oldestBlock.GetHeight())
			},
		},
		{
			name:           "should return full validity window built partially from cache and peers",
			validityWindow: 15,
			numOfBlocks:    20,
			setupChainIndex: func(blkChain []ExecutionBlock[container]) *testChainIndex {
				// Add the most recent 5 blocks in-memory
				return newTestChainIndex(blkChain[15:])
			},
			setupFetcher: newFetcher,
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], _ *testChainIndex) {
				// we should have the most recent 5 blocks in-memory
				// that is not enough to build full validity window, we need to fetch the rest from the network
				target := blkChain[len(blkChain)-1]
				err := syncer.Start(ctx, target)
				r.NoError(err)
				r.NoError(syncer.Wait(ctx))

				// the last acceptedIndex height should be the last acceptedIndex height from the cache, since historical blocks should not update the last acceptedIndex field
				r.Equal(blkChain[len(blkChain)-1].GetHeight(), syncer.timeValidityWindow.lastAcceptedBlockHeight)
				r.Equal(blkChain[15].GetHeight(), syncer.oldestBlock.GetHeight())

				// verify the oldest allowed block in time validity window
				r.Equal(blkChain[4].GetTimestamp(), syncer.timeValidityWindow.calculateOldestAllowed(target.GetTimestamp()))
				r.NotEqual(blkChain[3].GetTimestamp(), syncer.timeValidityWindow.calculateOldestAllowed(target.GetTimestamp()))
			},
		},
		{
			name:           "should return full validity window from peers",
			validityWindow: 15,
			numOfBlocks:    20,
			setupChainIndex: func(_ []ExecutionBlock[container]) *testChainIndex {
				return &testChainIndex{}
			},
			setupFetcher: newFetcher,
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], blockStore *testChainIndex) {
				target := blkChain[len(blkChain)-1]
				err := syncer.Start(ctx, target)
				r.NoError(err)
				r.NoError(syncer.Wait(ctx))

				r.Equal(uint64(19), syncer.timeValidityWindow.lastAcceptedBlockHeight)
				r.Equal(syncer.timeValidityWindow.calculateOldestAllowed(target.GetTimestamp()), blkChain[4].GetTimestamp())

				// Ensure historical (fetched) blocks are saved
				// this is required to ensure consistency between on-disk representation and validity window
				for i := 3; i < 19; i++ {
					blk := blkChain[i]
					fetchedBlk, err := blockStore.GetExecutionBlock(ctx, blk.GetID())
					r.NoError(err)
					r.Equal(blk, fetchedBlk)
				}
			},
		},
		{
			name:           "should stop historical block fetching in case of persistence error",
			validityWindow: 15,
			numOfBlocks:    20,
			setupChainIndex: func(_ []ExecutionBlock[container]) *testChainIndex {
				return &testChainIndex{
					beforeSaveFunc: func(_ map[ids.ID]ExecutionBlock[container]) error {
						return errSaveHistoricalBlocks
					},
				}
			},
			setupFetcher: newFetcher,
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], blockStore *testChainIndex) {
				target := blkChain[len(blkChain)-1]
				err := syncer.Start(ctx, target)
				r.NoError(err)
				r.ErrorIs(syncer.Wait(ctx), errSaveHistoricalBlocks)

				// Last accepted block in time validity window should be the target
				r.Equal(uint64(19), syncer.timeValidityWindow.lastAcceptedBlockHeight)

				// No blocks should be saved on chain index starting from target-1
				for i := 3; i < 19; i++ {
					blk := blkChain[i]
					_, err := blockStore.GetExecutionBlock(ctx, blk.GetID())
					r.Error(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSyncerTest(t, test)
		})
	}
}

func TestSyncer_UpdateSyncTarget(t *testing.T) {
	tests := []testCases{
		{
			name:           "update with newer block expands window forward",
			validityWindow: 15,
			numOfBlocks:    25,
			setupChainIndex: func(blkChain []ExecutionBlock[container]) *testChainIndex {
				// Start with the most recent 5 blocks in cache
				return newTestChainIndex(blkChain[20:])
			},
			setupFetcher: newFetcher,
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], _ *testChainIndex) {
				// Perform initial sync with second-to-last block
				initialTarget := blkChain[len(blkChain)-2]
				err := syncer.Start(ctx, initialTarget)
				r.NoError(err)
				r.NoError(syncer.Wait(ctx))

				initialMinTS := syncer.minTimestamp.Load()
				initialOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(initialTarget.GetTimestamp())

				// Update to newer block (the last block in chain)
				newTarget := blkChain[len(blkChain)-1]
				err = syncer.UpdateSyncTarget(ctx, newTarget)
				r.NoError(err)

				// Verify window has moved forward
				newOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(newTarget.GetTimestamp())
				r.Greater(newOldestAllowed, initialOldestAllowed, "window should expand forward with newer block")

				// minTimestamp defines the earliest point in time from which we need to maintain block history
				// When new blocks arrive from consensus, they effectively push this boundary forward in time
				// as newer blocks are added to the chain
				r.Greater(syncer.minTimestamp.Load(), initialMinTS, "min timestamp should move forward with newer block")
			},
		},
		{
			name:           "process sequence of consensus blocks maintains correct window",
			validityWindow: 15,
			numOfBlocks:    25,
			setupChainIndex: func(blkChain []ExecutionBlock[container]) *testChainIndex {
				return newTestChainIndex(blkChain[20:])
			},
			setupFetcher: newFetcher,
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], _ *testChainIndex) {
				// Start initial sync
				initialTarget := blkChain[len(blkChain)-1]
				err := syncer.Start(ctx, initialTarget)
				r.NoError(err)
				r.NoError(syncer.Wait(ctx))

				initialOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(initialTarget.GetTimestamp())

				// Simulate processing 5 new blocks from consensus
				currentTimestamp := initialTarget.GetTimestamp()
				currentHeight := initialTarget.GetHeight()

				for i := 0; i < 5; i++ {
					newBlock := newExecutionBlock(
						currentHeight+1,
						currentTimestamp+1,
						[]int64{},
					)
					// Update sync target
					err = syncer.UpdateSyncTarget(ctx, newBlock)
					r.NoError(err)

					// Verify window boundaries
					newOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(newBlock.GetTimestamp())
					r.Greater(
						newOldestAllowed,
						initialOldestAllowed,
						"window should move forward with each consensus block",
					)

					currentTimestamp = newBlock.GetTimestamp()
					currentHeight = newBlock.GetHeight()
				}
			},
		},
		{
			name:           "update with block at same height maintains window",
			validityWindow: 15,
			numOfBlocks:    25,
			setupChainIndex: func(blkChain []ExecutionBlock[container]) *testChainIndex {
				return newTestChainIndex(blkChain[20:])
			},
			setupFetcher: newFetcher,
			verify: func(ctx context.Context, r *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]], _ *testChainIndex) {
				// Start initial sync
				initialTarget := blkChain[len(blkChain)-1]
				err := syncer.Start(ctx, initialTarget)
				r.NoError(err)
				r.NoError(syncer.Wait(ctx))

				initialOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(initialTarget.GetTimestamp())

				// Create new block at same height but different ID
				sameHeightBlock := newExecutionBlock(
					initialTarget.GetHeight(),
					initialTarget.GetTimestamp(),
					[]int64{1}, // Different container to get different ID
				)

				// Update to new block
				err = syncer.UpdateSyncTarget(ctx, sameHeightBlock)
				r.NoError(err)

				// Verify window remains unchanged
				newOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(sameHeightBlock.GetTimestamp())
				r.Equal(initialOldestAllowed, newOldestAllowed, "window should not change with same-height block")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runSyncerTest(t, test)
		})
	}
}

func runSyncerTest(t *testing.T, test testCases) {
	ctx := context.Background()
	r := require.New(t)

	blkChain := generateTestChain(test.numOfBlocks)
	chainIndex := test.setupChainIndex(blkChain)

	head := blkChain[len(blkChain)-1]
	validityWindow, err := NewTimeValidityWindow(
		ctx,
		&logging.NoLog{},
		trace.Noop,
		chainIndex,
		head,
		func(_ int64) int64 { return test.validityWindow },
	)
	r.NoError(err)

	fetcher := test.setupFetcher(blkChain)
	syncer := NewSyncer[container, ExecutionBlock[container]](
		chainIndex,
		validityWindow,
		fetcher,
		func(_ int64) int64 { return test.validityWindow },
	)

	test.verify(ctx, r, blkChain, syncer, chainIndex)
}

func newTestChainIndex(blocks []ExecutionBlock[container]) *testChainIndex {
	ci := &testChainIndex{}
	for _, blk := range blocks {
		ci.set(blk.GetID(), blk)
	}
	return ci
}

func newFetcher(blkChain []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
	blocks := make(map[uint64]ExecutionBlock[container], len(blkChain))
	for _, blk := range blkChain {
		blocks[blk.GetHeight()] = blk
	}

	nodes := []nodeSetup{
		{blocks: blocks},
	}

	test := newTestEnvironment(nodes)
	blkParser := newParser(blkChain)
	return NewBlockFetcherClient[ExecutionBlock[container]](test.p2pBlockFetcher, blkParser, test.sampler)
}
