// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
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
			name: "partial blocks with missing height",
			setupBlocks: func() map[uint64]ExecutionBlock[container] {
				blocks := generateBlockChain(10, 3)
				delete(blocks, uint64(7))
				return blocks
			},
			blockHeight:  9,
			minTimestamp: 0,
			wantErr:      true,
			wantBlocks:   2, // Should get block 9 and 8 before failing on 7
		},
		{
			name:         "zero height request",
			setupBlocks:  func() map[uint64]ExecutionBlock[container] { return generateBlockChain(10, 3) },
			blockHeight:  0,
			minTimestamp: 0,
			wantBlocks:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := require.New(t)

			blocks := tt.setupBlocks()
			retriever := newTestBlockRetriever[ExecutionBlock[container]]().
				withBlocks(blocks)

			if tt.delay > 0 {
				retriever.withDelay(tt.delay)
			}

			handler := NewBlockFetcherHandler(retriever)
			fetchedBlocks, err := handler.fetchBlocks(ctx, &BlockFetchRequest{
				BlockHeight:  tt.blockHeight,
				MinTimestamp: tt.minTimestamp,
			})

			if tt.wantErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			req.Len(fetchedBlocks, tt.wantBlocks)
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
			retriever := newTestBlockRetriever[ExecutionBlock[container]]().
				withBlocks(blocks)

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
	req := require.New(t)

	blocks := generateBlockChain(20, 1)
	retriever := newTestBlockRetriever[ExecutionBlock[container]]().
		withBlocks(blocks).
		withDelay(maxProcessingDuration + 10*time.Millisecond)

	handler := NewBlockFetcherHandler(retriever)

	type result struct {
		blocks   [][]byte
		err      error
		duration time.Duration
	}
	resultCh := make(chan result, 1)

	start := time.Now()
	go func() {
		fetchedBlockBytes, err := handler.fetchBlocks(ctx, &BlockFetchRequest{
			BlockHeight:  9,
			MinTimestamp: 3,
		})
		resultCh <- result{
			blocks:   fetchedBlockBytes,
			err:      err,
			duration: time.Since(start),
		}
	}()

	select {
	case r := <-resultCh:
		req.NoError(r.err)
		// We should get some blocks before timeout
		req.NotEmpty(r.blocks)
		// But not all; 7 = (Block of nextHeight 10, fetch until minTimestamp of block 3)
		req.NotEqual(7, len(r.blocks))
	case <-time.After(2 * maxProcessingDuration):
		req.Fail("Test took too long to complete")
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

var _ BlockRetriever[Block] = (*testBlockRetriever[Block])(nil)

type testBlockRetriever[T Block] struct {
	delay   time.Duration
	nodeID  ids.NodeID
	errChan chan error
	blocks  map[uint64]T
}

func newTestBlockRetriever[T Block]() *testBlockRetriever[T] {
	return &testBlockRetriever[T]{
		errChan: make(chan error, 1),
	}
}

func (r *testBlockRetriever[T]) withBlocks(blocks map[uint64]T) *testBlockRetriever[T] {
	r.blocks = blocks
	return r
}

func (r *testBlockRetriever[T]) withDelay(delay time.Duration) *testBlockRetriever[T] {
	r.delay = delay
	return r
}

func (r *testBlockRetriever[T]) withNodeID(nodeID ids.NodeID) *testBlockRetriever[T] {
	r.nodeID = nodeID
	return r
}

func (r *testBlockRetriever[T]) GetBlockByHeight(_ context.Context, blockHeight uint64) (T, error) {
	if r.delay.Milliseconds() > 0 {
		time.Sleep(r.delay)
	}

	var err error
	if r.nodeID.Compare(ids.EmptyNodeID) == 0 {
		err = fmt.Errorf("block nextHeight %d not found", blockHeight)
	} else {
		err = fmt.Errorf("%s: block nextHeight %d not found", r.nodeID, blockHeight)
	}

	block, ok := r.blocks[blockHeight]

	if !ok && err != nil {
		r.errChan <- err
		return utils.Zero[T](), err
	}
	return block, nil
}
