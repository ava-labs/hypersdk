// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func TestBlockFetcherHandler_FetchBlocks(t *testing.T) {
	tests := []struct {
		name           string
		setupBlocks    func() map[uint64]ExecutionBlock[container]
		blockHeight    uint64
		minTimestamp   int64
		expectErr      bool
		expectedBlocks int
		blkControl     chan struct{} // control the number of returned blocks
	}{
		{
			name: "happy path - fetch all blocks",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(10, 3)
			},
			blockHeight:    9,
			minTimestamp:   3,
			expectedBlocks: 8,
		},
		{
			name: "partial response",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				blocks := generateBlockChain(10, 3)
				delete(blocks, uint64(7))
				return blocks
			},
			blockHeight:    9,
			minTimestamp:   0,
			expectedBlocks: 2,
		},
		{
			name:           "no blocks - should error",
			setupBlocks:    func() map[uint64]ExecutionBlock[container] { return nil },
			blockHeight:    9,
			minTimestamp:   3,
			expectedBlocks: 0,
			expectErr:      true,
		},
		{
			name: "should error if block does not exist",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(2, 1)
			},
			blockHeight:  4,
			minTimestamp: 3,
			expectErr:    true,
		},
		{
			// Tests timeout behavior by retrieving exactly 3 blocks before canceling the context.
			// When the context is canceled, it is expected to return a partial response
			name: "should timeout",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(10, 3)
			},
			blockHeight:    9,
			minTimestamp:   3,
			expectedBlocks: 3,                      // Expect to retrieve exactly 3 blocks before context cancellation
			blkControl:     make(chan struct{}, 3), // Used to trigger context cancellation after 3 blocks
			// No expectErr flag as we don't treat partial results as an error case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			r := require.New(t)

			blocks := tt.setupBlocks()

			fetchCtx, cancel := context.WithTimeout(ctx, maxProcessingDuration)
			defer cancel()

			retriever := newTestBlockRetriever(withBlocks(blocks))
			if tt.blkControl != nil {
				retriever = newTestBlockRetriever(withBlocks(blocks), withBlockControl(tt.blkControl), withCtxCancelFunc(cancel))
			}

			handler := NewBlockFetcherHandler(retriever)
			fetchedBlocks, err := handler.fetchBlocks(fetchCtx, &BlockFetchRequest{
				BlockHeight:  tt.blockHeight,
				MinTimestamp: tt.minTimestamp,
			})

			if tt.expectErr {
				r.Error(err)
			} else {
				r.NoError(err)
				r.Len(fetchedBlocks, tt.expectedBlocks)
			}

			for _, blk := range fetchedBlocks {
				parsedBlk, err := retriever.parseBlock(blk)
				r.NoError(err)

				expectedBlock, ok := blocks[parsedBlk.GetHeight()]
				r.True(ok)
				r.EqualValues(expectedBlock, parsedBlk)
			}
		})
	}
}

func generateBlockChain(n int, containersPerBlock int) map[uint64]ExecutionBlock[container] {
	genesis := newExecutionBlock(0, 0, []int64{})
	blks := make(map[uint64]ExecutionBlock[container])
	blks[0] = genesis

	for i := 1; i < n; i++ {
		containers := make([]int64, containersPerBlock)
		for j := 0; j < containersPerBlock; j++ {
			expiry := int64((i * containersPerBlock) + j + 1)
			containers[j] = expiry
		}
		b := newExecutionBlock(uint64(i), int64(i), containers)
		b.Prnt = blks[uint64(i-1)].GetID()
		blks[uint64(i)] = b
	}

	return blks
}

type testBlockRetriever struct {
	options *optionsImpl
	errChan chan error
}

func newTestBlockRetriever(opts ...option) *testBlockRetriever {
	optImpl := &optionsImpl{
		nodeID: ids.EmptyNodeID,
		blocks: make(map[uint64]ExecutionBlock[container]),
	}

	// Apply all provided options
	for _, opt := range opts {
		if opt != nil {
			opt.apply(optImpl)
		}
	}

	retriever := &testBlockRetriever{
		options: optImpl,
		errChan: make(chan error, 1),
	}

	return retriever
}

type optionsImpl struct {
	nodeID       ids.NodeID
	cancel       context.CancelFunc
	blockControl chan struct{}
	blocks       map[uint64]ExecutionBlock[container]
}

type option interface {
	apply(*optionsImpl)
}

// NodeID option
type nodeIDOption ids.NodeID

func (n nodeIDOption) apply(opts *optionsImpl) {
	opts.nodeID = ids.NodeID(n)
}

func withNodeID(nodeID ids.NodeID) option {
	return nodeIDOption(nodeID)
}

// Blocks option
type blocksOption map[uint64]ExecutionBlock[container]

func (b blocksOption) apply(opts *optionsImpl) {
	opts.blocks = b
}

func withBlocks(blocks map[uint64]ExecutionBlock[container]) option {
	return blocksOption(blocks)
}

type blockControl chan struct{}

func (b blockControl) apply(opts *optionsImpl) {
	opts.blockControl = b
}

func withBlockControl(buffer chan struct{}) option {
	if buffer != nil {
		return blockControl(buffer)
	}

	return nil
}

type ctxCancelFunc context.CancelFunc

func (c ctxCancelFunc) apply(opts *optionsImpl) {
	opts.cancel = context.CancelFunc(c)
}

func withCtxCancelFunc(cancel context.CancelFunc) option {
	return ctxCancelFunc(cancel)
}

func (r *testBlockRetriever) GetBlockByHeight(ctx context.Context, blockHeight uint64) (ExecutionBlock[container], error) {
	if r.options.blockControl != nil {
		select {
		case r.options.blockControl <- struct{}{}:
			// Simulate context cancellation after N blocks for deterministic testing:
			// - Channel tracks number of retrieved blocks
			// - When a channel reaches capacity, trigger context cancellation
			// - Avoids flaky timeout tests while testing the same code path

			if len(r.options.blockControl) >= cap(r.options.blockControl) && r.options.cancel != nil {
				// Cancel context after reaching channel capacity
				r.options.cancel()
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	block, ok := r.options.blocks[blockHeight]
	if !ok {
		return nil, fmt.Errorf("%s: block height %d not found", r.options.nodeID, blockHeight)
	}
	return block, nil
}

func (r *testBlockRetriever) parseBlock(blk []byte) (ExecutionBlock[container], error) {
	// Ensure we have at least 8 bytes for the height
	if len(blk) < 8 {
		return nil, fmt.Errorf("block bytes too short: %d bytes", len(blk))
	}

	// Extract the height from the first 8 bytes
	height := binary.BigEndian.Uint64(blk[:8])

	for _, block := range r.options.blocks {
		if block.GetHeight() == height {
			return block, nil
		}
	}

	return nil, fmt.Errorf("block with height %d not found", height)
}
