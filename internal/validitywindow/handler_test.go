// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"
)

func TestBlockFetcherHandler_FetchBlocks(t *testing.T) {
	tests := []struct {
		name         string
		setupBlocks  func() map[uint64]ExecutionBlock[container]
		blockHeight  uint64
		minTimestamp int64
		delay        time.Duration
		wantErr      bool
		wantBlocks   int
	}{
		{
			name: "happy path - fetch all blocks",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				return generateBlockChain(10, 3)
			},
			blockHeight:  9, // The tip of the chain
			minTimestamp: 3, // Should get blocks with timestamp of 3 - 1 = 2 where 2 is the boundary block
			wantBlocks:   8,
		},
		{
			name: "partial response",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				blocks := generateBlockChain(10, 3)
				delete(blocks, uint64(7))
				return blocks
			},
			blockHeight:  9,
			minTimestamp: 0,
			wantBlocks:   2, // Should get block 9 and 8 before failing on 7
		},
		{
			name:         "no blocks - should error",
			setupBlocks:  func() map[uint64]ExecutionBlock[container] { return nil },
			blockHeight:  9,
			minTimestamp: 3,
			wantBlocks:   0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			r := require.New(t)

			blocks := tt.setupBlocks()
			retriever := newTestBlockRetriever(withBlocks(blocks), withDelay(tt.delay))

			handler := NewBlockFetcherHandler(retriever)
			fetchedBlocks, err := handler.fetchBlocks(ctx, &BlockFetchRequest{
				BlockHeight:  tt.blockHeight,
				MinTimestamp: tt.minTimestamp,
			})

			if tt.wantErr {
				r.Error(err)
			} else {
				r.NoError(err)
			}

			r.Len(fetchedBlocks, tt.wantBlocks)
			for _, blk := range fetchedBlocks {
				block, err := retriever.parseBlock(blk)
				r.NoError(err)

				_, ok := blocks[block.GetHeight()]
				r.True(ok)
			}
		})
	}
}

func TestBlockFetcherHandler_AppRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     []byte
		wantErrCode int32
	}{
		{
			name:        "invalid request bytes",
			request:     []byte("invalid"),
			wantErrCode: ErrCodeUnmarshal,
		},
		{
			name: "valid request",
			request: (&BlockFetchRequest{
				BlockHeight:  9,
				MinTimestamp: 3,
			}).MarshalCanoto(),
			wantErrCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := require.New(t)

			blocks := generateBlockChain(10, 5)
			retriever := newTestBlockRetriever(withBlocks(blocks))

			handler := NewBlockFetcherHandler(retriever)

			response, appErr := handler.AppRequest(
				ctx,
				ids.EmptyNodeID,
				time.Now(),
				tt.request,
			)

			if tt.wantErrCode > 0 {
				req.NotNil(appErr)
				req.Equal(tt.wantErrCode, appErr.Code)
			} else {
				req.Nil(appErr)
				req.NotEmpty(response)

				// Verify response can be unmarshaled
				resp := new(BlockFetchResponse)
				err := resp.UnmarshalCanoto(response)
				req.NoError(err)
				req.NotEmpty(resp.Blocks)
			}
		})
	}
}

func TestBlockFetcherHandler_Timeout(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)

	blks := generateBlockChain(20, 1)
	retriever := newTestBlockRetriever(withBlocks(blks), withDelay(maxProcessingDuration+10*time.Millisecond))

	handler := NewBlockFetcherHandler(retriever)
	fetchedBlocksBytes, err := handler.fetchBlocks(ctx, &BlockFetchRequest{
		BlockHeight:  9,
		MinTimestamp: 3,
	})

	r.NoError(err)
	// We should not receive all blocks from (9 to 2) due to delay which should trigger timeout
	r.NotEqual(7, len(fetchedBlocksBytes))
	// We should've received some blocks
	r.NotEmpty(fetchedBlocksBytes)

	// Verify received block bytes when parsed exist in retriever's state
	for _, blkBytes := range fetchedBlocksBytes {
		_, err := retriever.parseBlock(blkBytes)
		r.NoError(err)
	}
}

func generateBlockChain(n int, containersPerBlock int) map[uint64]ExecutionBlock[container] {
	genesis := newExecutionBlock(0, 0, []int64{})
	blks := make(map[uint64]ExecutionBlock[container])
	blks[0] = genesis

	for i := 1; i < n; i++ {
		containers := make([]int64, containersPerBlock)
		for j := 0; j < containersPerBlock; j++ {
			containers[j] = rand.Int63n(int64(n)) //nolint:gosec
		}
		b := newExecutionBlock(uint64(i), int64(i), containers)
		b.Prnt = blks[uint64(i-1)].GetID()
		blks[uint64(i)] = newExecutionBlock(uint64(i), int64(i), containers)
	}

	return blks
}

type testBlockRetriever struct {
	options *optionsImpl
	errChan chan error
}

func newTestBlockRetriever(opts ...option) *testBlockRetriever {
	optImpl := &optionsImpl{
		delay:  0,
		nodeID: ids.EmptyNodeID,
		blocks: make(map[uint64]ExecutionBlock[container]),
	}

	// Apply all provided options
	for _, opt := range opts {
		if opt != nil {
			opt.apply(optImpl)
		}
	}

	return &testBlockRetriever{
		options: optImpl,
		errChan: make(chan error, 1),
	}
}

type optionsImpl struct {
	delay  time.Duration
	nodeID ids.NodeID
	blocks map[uint64]ExecutionBlock[container]
}

type option interface {
	apply(*optionsImpl)
}

// Delay option
type delayOption time.Duration

func (d delayOption) apply(opts *optionsImpl) {
	opts.delay = time.Duration(d)
}

func withDelay(delay time.Duration) option {
	if delay > 0 {
		return delayOption(delay)
	}

	return nil
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

func (r *testBlockRetriever) GetBlockByHeight(_ context.Context, blockHeight uint64) (ExecutionBlock[container], error) {
	if r.options.delay.Milliseconds() > 0 {
		time.Sleep(r.options.delay)
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
