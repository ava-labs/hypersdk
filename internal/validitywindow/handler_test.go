// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"
)

// Use a fixed seed for deterministic test results
var seed = rand.New(rand.NewSource(42)) //nolint:gosec

func TestBlockFetcherHandler_FetchBlocks(t *testing.T) {
	tests := []struct {
		name          string
		setupBlocks   func() map[uint64]ExecutionBlock[container]
		blockHeight   uint64
		minTimestamp  int64
		expectErr     bool
		wantBlocks    int
		controlBuffer func() chan struct{}
	}{
		{
			name: "happy path - fetch all blocks",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(10, 3, seed)
			},
			blockHeight:  9,
			minTimestamp: 3,
			wantBlocks:   8,
		},
		{
			name: "partial response",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				blocks := generateBlockChain(10, 3, seed)
				delete(blocks, uint64(7))
				return blocks
			},
			blockHeight:  9,
			minTimestamp: 0,
			wantBlocks:   2,
		},
		{
			name:         "no blocks - should error",
			setupBlocks:  func() map[uint64]ExecutionBlock[container] { return nil },
			blockHeight:  9,
			minTimestamp: 3,
			wantBlocks:   0,
			expectErr:    true,
		},
		{
			name: "should error if block does not exist",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(2, 1, seed)
			},
			blockHeight:  4,
			minTimestamp: 3,
			expectErr:    true,
		},
		{
			name: "should timeout",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(10, 3, seed)
			},
			blockHeight:  9,
			minTimestamp: 3,
			wantBlocks:   1,
			controlBuffer: func() chan struct{} {
				return make(chan struct{}, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			r := require.New(t)

			blocks := tt.setupBlocks()

			retriever := newTestBlockRetriever(withBlocks(blocks))
			if tt.controlBuffer != nil {
				retriever = newTestBlockRetriever(withBlocks(blocks), withBufferedReads(tt.controlBuffer()))
			}

			handler := NewBlockFetcherHandler(retriever)
			fetchCtx, cancel := context.WithTimeout(ctx, maxProcessingDuration)
			defer cancel()

			fetchedBlocks, err := handler.fetchBlocks(fetchCtx, &BlockFetchRequest{
				BlockHeight:  tt.blockHeight,
				MinTimestamp: tt.minTimestamp,
			})

			if tt.expectErr {
				r.Error(err)
			} else {
				r.NoError(err)
				r.Len(fetchedBlocks, tt.wantBlocks)
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

func generateBlockChain(n int, containersPerBlock int, r *rand.Rand) map[uint64]ExecutionBlock[container] {
	genesis := newExecutionBlock(0, 0, []int64{})
	blks := make(map[uint64]ExecutionBlock[container])
	blks[0] = genesis

	for i := 1; i < n; i++ {
		containers := make([]int64, containersPerBlock)
		for j := 0; j < containersPerBlock; j++ {
			containers[j] = r.Int63n(int64(n))
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
	nodeID      ids.NodeID
	blocks      map[uint64]ExecutionBlock[container]
	bufferReads chan struct{}
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

type bufferedReadsOption chan struct{}

func (b bufferedReadsOption) apply(opts *optionsImpl) {
	opts.bufferReads = b
}

func withBufferedReads(buffer chan struct{}) option {
	if buffer != nil {
		return bufferedReadsOption(buffer)
	}

	return nil
}

func (r *testBlockRetriever) GetBlockByHeight(ctx context.Context, blockHeight uint64) (ExecutionBlock[container], error) {
	if r.options.bufferReads != nil {
		select {
		case r.options.bufferReads <- struct{}{}:
		case <-ctx.Done():
			// Context canceled, return error
			return utils.Zero[ExecutionBlock[container]](), ctx.Err()
		}
	}

	block, ok := r.options.blocks[blockHeight]
	if !ok {
		return utils.Zero[ExecutionBlock[container]](), fmt.Errorf("%s: block height %d not found", r.options.nodeID, blockHeight)
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
