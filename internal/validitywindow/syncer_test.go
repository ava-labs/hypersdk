// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

type testCases struct {
	name            string
	validityWindow  int64
	numOfBlocks     int
	setupChainIndex func([]ExecutionBlock[container]) *testChainIndex
	setupFetcher    func(context.Context, []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]]
	verifyFunc      func(context.Context, *require.Assertions, []ExecutionBlock[container], *Syncer[container, ExecutionBlock[container]])
}

func TestSyncer(t *testing.T) {
	t.Run("Start", func(t *testing.T) {
		tests := []testCases{
			{
				name:            "should return full validity window from cache",
				numOfBlocks:     15,
				validityWindow:  5,
				setupChainIndex: newTestChainIndex,
				setupFetcher: func(_ context.Context, _ []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
					// no need for fetcher
					return nil
				},
				verifyFunc: func(ctx context.Context, req *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]]) {
					target := blkChain[len(blkChain)-1]
					err := syncer.Start(ctx, target)
					req.NoError(err)
					req.Equal(blkChain[len(blkChain)-1].GetHeight(), syncer.timeValidityWindow.lastAcceptedBlockHeight)
					// We're expecting oldestBlock to have height 8 because:
					// - We have 15 blocks (height 0-14)
					// - Validity window is 5 time units
					// - Given target block at height 14 (timestamp 14)
					// - We need blocks until timestamp difference > 5
					// - This happens at block height 8 (14 - 8 > 5)
					req.Equal(blkChain[8].GetHeight(), syncer.oldestBlock.GetHeight())
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
				setupFetcher: func(ctx context.Context, blkChain []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
					blocks := make(map[uint64]ExecutionBlock[container], len(blkChain))
					for _, blk := range blkChain {
						blocks[blk.GetHeight()] = blk
					}

					nodes := []nodeScenario{
						{
							blocks: blocks,
						},
					}

					network := newTestNetwork(t, ctx, nodes)
					blkParser := newParser(blkChain)
					fetcher := NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)

					return fetcher
				},
				verifyFunc: func(ctx context.Context, req *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]]) {
					// we should have the most recent 5 blocks in-memory
					// that is not enough to build full validity window, we need to fetch the rest from the network
					target := blkChain[len(blkChain)-1]
					err := syncer.Start(ctx, target)
					req.NoError(err)
					req.NoError(syncer.Wait(ctx))

					// the last accepted height should be the last accepted height from the cache, since historical blocks should not update the last accepted field
					req.Equal(blkChain[len(blkChain)-1].GetHeight(), syncer.timeValidityWindow.lastAcceptedBlockHeight)
					req.Equal(blkChain[15].GetHeight(), syncer.oldestBlock.GetHeight())

					// verify the oldest allowed block in time validity window
					req.Equal(blkChain[4].GetTimestamp(), syncer.timeValidityWindow.calculateOldestAllowed(target.GetTimestamp()))
					req.NotEqual(blkChain[3].GetTimestamp(), syncer.timeValidityWindow.calculateOldestAllowed(target.GetTimestamp()))
				},
			},
			{
				name:           "should return full validity window from peers",
				validityWindow: 15,
				numOfBlocks:    20,
				setupChainIndex: func(_ []ExecutionBlock[container]) *testChainIndex {
					return &testChainIndex{}
				},
				setupFetcher: func(ctx context.Context, blkChain []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
					blocks := make(map[uint64]ExecutionBlock[container], len(blkChain))
					for _, blk := range blkChain {
						blocks[blk.GetHeight()] = blk
					}

					nodes := []nodeScenario{
						{
							blocks: blocks,
						},
					}

					network := newTestNetwork(t, ctx, nodes)
					blkParser := newParser(blkChain)
					fetcher := NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)

					return fetcher
				},
				verifyFunc: func(ctx context.Context, req *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]]) {
					target := blkChain[len(blkChain)-1]
					err := syncer.Start(ctx, target)
					req.NoError(err)
					req.NoError(syncer.Wait(ctx))

					req.Equal(uint64(19), syncer.timeValidityWindow.lastAcceptedBlockHeight)
					req.Equal(syncer.timeValidityWindow.calculateOldestAllowed(target.GetTimestamp()), blkChain[4].GetTimestamp())
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				runSyncerTest(t, test)
			})
		}
	})

	t.Run("UpdateSyncTarget", func(t *testing.T) {
		tests := []testCases{
			{
				name:           "update with newer block expands window forward",
				validityWindow: 15,
				numOfBlocks:    25,
				setupChainIndex: func(blkChain []ExecutionBlock[container]) *testChainIndex {
					// Start with the most recent 5 blocks in cache
					return newTestChainIndex(blkChain[20:])
				},
				setupFetcher: func(ctx context.Context, blkChain []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
					blocks := make(map[uint64]ExecutionBlock[container])
					for _, blk := range blkChain {
						blocks[blk.GetHeight()] = blk
					}

					nodes := []nodeScenario{
						{blocks: blocks},
					}

					network := newTestNetwork(t, ctx, nodes)
					blkParser := newParser(blkChain)
					return NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)
				},
				verifyFunc: func(ctx context.Context, req *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]]) {
					// Perform initial sync with second-to-last block
					initialTarget := blkChain[len(blkChain)-2]
					err := syncer.Start(ctx, initialTarget)
					req.NoError(err)
					req.NoError(syncer.Wait(ctx))

					initialMinTS := syncer.minTimestamp.Load()
					initialOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(initialTarget.GetTimestamp())

					// Update to newer block (the last block in chain)
					newTarget := blkChain[len(blkChain)-1]
					err = syncer.UpdateSyncTarget(ctx, newTarget)
					req.NoError(err)

					// Verify window has moved forward
					newOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(newTarget.GetTimestamp())
					req.Greater(newOldestAllowed, initialOldestAllowed, "window should expand forward with newer block")

					// minTimestamp defines the earliest point in time from which we need to maintain block history
					// When new blocks arrive from consensus, they effectively push this boundary forward in time
					// as newer blocks are added to the chain
					req.Greater(syncer.minTimestamp.Load(), initialMinTS, "min timestamp should move forward with newer block")
				},
			},
			{
				name:           "process sequence of consensus blocks maintains correct window",
				validityWindow: 15,
				numOfBlocks:    25,
				setupChainIndex: func(blkChain []ExecutionBlock[container]) *testChainIndex {
					return newTestChainIndex(blkChain[20:])
				},
				setupFetcher: func(ctx context.Context, blkChain []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
					blocks := make(map[uint64]ExecutionBlock[container])
					for _, blk := range blkChain {
						blocks[blk.GetHeight()] = blk
					}

					nodes := []nodeScenario{
						{blocks: blocks},
					}

					network := newTestNetwork(t, ctx, nodes)
					blkParser := newParser(blkChain)
					return NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)
				},
				verifyFunc: func(ctx context.Context, req *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]]) {
					// Start initial sync
					initialTarget := blkChain[len(blkChain)-1]
					err := syncer.Start(ctx, initialTarget)
					req.NoError(err)
					req.NoError(syncer.Wait(ctx))

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
						req.NoError(err)

						// Verify window boundaries
						newOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(newBlock.GetTimestamp())
						req.Greater(
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
				setupFetcher: func(ctx context.Context, blkChain []ExecutionBlock[container]) *BlockFetcherClient[ExecutionBlock[container]] {
					blocks := make(map[uint64]ExecutionBlock[container])
					for _, blk := range blkChain {
						blocks[blk.GetHeight()] = blk
					}

					nodes := []nodeScenario{
						{blocks: blocks},
					}

					network := newTestNetwork(t, ctx, nodes)
					blkParser := newParser(blkChain)
					return NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)
				},
				verifyFunc: func(ctx context.Context, req *require.Assertions, blkChain []ExecutionBlock[container], syncer *Syncer[container, ExecutionBlock[container]]) {
					// Start initial sync
					initialTarget := blkChain[len(blkChain)-1]
					err := syncer.Start(ctx, initialTarget)
					req.NoError(err)
					req.NoError(syncer.Wait(ctx))

					initialOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(initialTarget.GetTimestamp())

					// Create new block at same height but different ID
					sameHeightBlock := newExecutionBlock(
						initialTarget.GetHeight(),
						initialTarget.GetTimestamp(),
						[]int64{1}, // Different container to get different ID
					)

					// Update to new block
					err = syncer.UpdateSyncTarget(ctx, sameHeightBlock)
					req.NoError(err)

					// Verify window remains unchanged
					newOldestAllowed := syncer.timeValidityWindow.calculateOldestAllowed(sameHeightBlock.GetTimestamp())
					req.Equal(initialOldestAllowed, newOldestAllowed, "window should not change with same-height block")
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				runSyncerTest(t, test)
			})
		}
	})
}

func runSyncerTest(t *testing.T, test testCases) {
	ctx := context.Background()
	req := require.New(t)

	blkChain := generateTestChain(test.numOfBlocks)
	chainIndex := test.setupChainIndex(blkChain)

	validityWindow := NewTimeValidityWindow(
		&logging.NoLog{},
		trace.Noop,
		chainIndex,
		func(_ int64) int64 { return test.validityWindow },
	)

	fetcher := test.setupFetcher(ctx, blkChain)
	syncer := NewSyncer[container, ExecutionBlock[container]](
		chainIndex,
		validityWindow,
		fetcher,
		func(_ int64) int64 { return test.validityWindow },
	)

	test.verifyFunc(ctx, req, blkChain, syncer)
}

func newTestChainIndex(blocks []ExecutionBlock[container]) *testChainIndex {
	ci := &testChainIndex{}
	for _, blk := range blocks {
		ci.set(blk.GetID(), blk)
	}
	return ci
}
