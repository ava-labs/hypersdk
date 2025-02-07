package blockwindowsyncer

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/blockwindowsyncer/blockwindowsyncertest"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestBlockFetcherHandler_FetchBlocks(t *testing.T) {
	tests := []struct {
		name         string
		setupBlocks  func() (map[uint64]*blockwindowsyncertest.TestBlock, []*blockwindowsyncertest.TestBlock)
		blockHeight  uint64
		minTimestamp int64
		delay        time.Duration
		wantErr      bool
		wantBlocks   int
	}{
		{
			name: "happy path - fetch all blocks",
			setupBlocks: func() (map[uint64]*blockwindowsyncertest.TestBlock, []*blockwindowsyncertest.TestBlock) {
				return generateBlocks(10)
			},
			blockHeight:  9, // The tip of the chain
			minTimestamp: 3, // Should get blocks with timestamp >= 3
			wantBlocks:   7,
		},
		{
			name: "partial blocks with missing height",
			setupBlocks: func() (map[uint64]*blockwindowsyncertest.TestBlock, []*blockwindowsyncertest.TestBlock) {
				blocks, chain := generateBlocks(10)
				delete(blocks, uint64(7))
				return blocks, chain
			},
			blockHeight:  9,
			minTimestamp: 0,
			wantErr:      true,
			wantBlocks:   2, // Should get block 9 and 8 before failing on 7
		},
		{
			name: "zero height request",
			setupBlocks: func() (map[uint64]*blockwindowsyncertest.TestBlock, []*blockwindowsyncertest.TestBlock) {
				return generateBlocks(10)
			},
			blockHeight:  0,
			minTimestamp: 0,
			wantBlocks:   0,
		},
		{
			name: "future timestamp",
			setupBlocks: func() (map[uint64]*blockwindowsyncertest.TestBlock, []*blockwindowsyncertest.TestBlock) {
				return generateBlocks(10)
			},
			blockHeight:  9,
			minTimestamp: time.Now().Unix() + 1000,
			wantBlocks:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := require.New(t)

			blocks, _ := tt.setupBlocks()
			retriever := blockwindowsyncertest.NewTestBlockRetriever().
				WithBlocks(blocks)

			if tt.delay > 0 {
				retriever.WithDelay(tt.delay)
			}

			handler := NewBlockFetcherHandler[*blockwindowsyncertest.TestBlock](retriever)
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

			blocks, _ := generateBlocks(10)
			retriever := blockwindowsyncertest.NewTestBlockRetriever().
				WithBlocks(blocks)

			handler := NewBlockFetcherHandler[*blockwindowsyncertest.TestBlock](retriever)

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

func TestBlockFetcherHandler_Context(t *testing.T) {
	ctx := context.Background()
	req := require.New(t)

	blocks, _ := generateBlocks(10)
	retriever := blockwindowsyncertest.NewTestBlockRetriever().
		WithBlocks(blocks).
		WithDelay(10 * time.Millisecond) // Add some delay to ensure cancellation works

	handler := NewBlockFetcherHandler[*blockwindowsyncertest.TestBlock](retriever)

	ctx, cancel := context.WithCancel(ctx)

	// Start fetching in a goroutine
	resultCh := make(chan struct {
		blocks [][]byte
		err    error
	})

	go func() {
		fetchedBlockBytes, err := handler.fetchBlocks(ctx, &BlockFetchRequest{
			BlockHeight:  9,
			MinTimestamp: 0,
		})
		resultCh <- struct {
			blocks [][]byte
			err    error
		}{fetchedBlockBytes, err}
	}()

	// Cancel the context after a short delay
	time.Sleep(25 * time.Millisecond)
	cancel()

	// Wait for result
	result := <-resultCh
	req.Error(result.err)
	req.ErrorIs(result.err, context.Canceled)
}

func TestBlockFetcherHandler_Timeout(t *testing.T) {
	ctx := context.Background()
	req := require.New(t)

	blocks, _ := generateBlocks(10)
	retriever := blockwindowsyncertest.NewTestBlockRetriever().
		WithBlocks(blocks).
		WithDelay(maxProcessingDuration + 10*time.Millisecond)

	handler := NewBlockFetcherHandler[*blockwindowsyncertest.TestBlock](retriever)

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
		req.Error(r.err)
		req.ErrorIs(r.err, context.DeadlineExceeded)
		// We should get some blocks before timeout
		req.NotEmpty(r.blocks)
		// But not all; 7 = (Block of height 10, fetch until timestamp of block 3)
		req.NotEqual(len(r.blocks), 7)
	case <-time.After(2 * maxProcessingDuration):
		t.Fatal("Test took too long to complete")
	}
}

func generateBlocks(num int) (map[uint64]*blockwindowsyncertest.TestBlock, []*blockwindowsyncertest.TestBlock) {
	blocks := make(map[uint64]*blockwindowsyncertest.TestBlock, num)
	chain := blockwindowsyncertest.GenerateChain(num)
	for _, block := range chain {
		blocks[block.GetHeight()] = block
	}

	return blocks, chain
}
