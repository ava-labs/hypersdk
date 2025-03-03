// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/hypersdk/internal/typedclient"
)

const (
	requestTimeout = 1 * time.Second // Timeout for each request
	backoff        = 500 * time.Millisecond
)

type BlockParser[T Block] interface {
	ParseBlock(ctx context.Context, blockBytes []byte) (T, error)
}

// BlockFetcherClient fetches blocks from peers in a backward fashion (N, N-1, N-2, N-K) until it fills validity window of
// blocks, it ensures we have at least min validity window of blocks so we can transition from state sync to normal operation faster
type BlockFetcherClient[B Block] struct {
	parser     BlockParser[B]
	sampler    p2p.NodeSampler
	syncClient *typedclient.SyncTypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte]
	lastBlock  Block
	nextHeight uint64
}

func NewBlockFetcherClient[B Block](
	baseClient *p2p.Client,
	parser BlockParser[B],
	sampler p2p.NodeSampler,
) *BlockFetcherClient[B] {
	return &BlockFetcherClient[B]{
		syncClient: typedclient.NewSyncTypedClient(baseClient, &blockFetcherMarshaler{}),
		parser:     parser,
		sampler:    sampler,
	}
}

// FetchBlocks fetches blocks from peers **backward**.
//   - It stops when `minTS` is met (this can update dynamically, e.g., via `UpdateSyncTarget`).
//   - Each request is limited by the node's max execution time (currently ~50ms),
//     meaning multiple requests may be needed to retrieve all required blocks.
//   - If a peer is unresponsive or sends bad data, we retry with another
func (c *BlockFetcherClient[B]) FetchBlocks(ctx context.Context, blk Block, minTimestamp *atomic.Int64) <-chan FetchResult[B] {
	resultChan := make(chan FetchResult[B], 1)

	go func() {
		c.lastBlock = blk
		req := &BlockFetchRequest{MinTimestamp: minTimestamp.Load(), BlockHeight: blk.GetHeight() - 1}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Multiple blocks can share the same timestamp, so we have not filled the validity window
			// until we find and include the first block whose timestamp is strictly less than the minimum
			// timestamp. This ensures we have a complete and verifiable validity window
			if c.lastBlock.GetTimestamp() < minTimestamp.Load() {
				close(resultChan)
				return
			}

			nodeID, ok := c.sampleNodeID(ctx)
			if !ok {
				time.Sleep(backoff)
				continue
			}

			reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
			response, err := c.syncClient.SyncAppRequest(reqCtx, nodeID, req)
			cancel()

			if err != nil || response == nil {
				time.Sleep(backoff)
				continue
			}

			expectedParentID := c.lastBlock.GetParent()
			for _, raw := range response.Blocks {
				block, err := c.parser.ParseBlock(ctx, raw)
				if err != nil {
					continue
				}

				if expectedParentID != block.GetID() {
					continue
				}
				expectedParentID = block.GetParent()

				select {
				case <-ctx.Done():
					return
				case resultChan <- FetchResult[B]{Block: block}:
					c.lastBlock = block
					req.BlockHeight = c.lastBlock.GetHeight() - 1
				}
			}
			time.Sleep(backoff)
		}
	}()

	return resultChan
}

func (c *BlockFetcherClient[B]) sampleNodeID(ctx context.Context) (ids.NodeID, bool) {
	nodes := c.sampler.Sample(ctx, 1)
	if len(nodes) == 0 {
		return ids.EmptyNodeID, false
	}
	return nodes[0], true
}
