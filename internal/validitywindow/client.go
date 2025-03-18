// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
)

const (
	requestTimeout = 1 * time.Second // Timeout for each request
	backoff        = 500 * time.Millisecond
)

type BlockParser[T Block] interface {
	ParseBlock(ctx context.Context, blockBytes []byte) (T, error)
}

type NetworkBlockFetcher interface {
	FetchBlocksFromPeer(ctx context.Context, peerID ids.NodeID, request *BlockFetchRequest) (*BlockFetchResponse, error)
}

// BlockFetcherClient fetches blocks from peers in a backward fashion (N, N-1, N-2, N-K) until it fills validity window of
// blocks, it ensures we have at least min validity window of blocks so we can transition from state sync to normal operation faster
type BlockFetcherClient[B Block] struct {
	parser          BlockParser[B]
	p2pBlockFetcher NetworkBlockFetcher
	lastBlock       Block
	sampler         p2p.NodeSampler
}

func NewBlockFetcherClient[B Block](
	p2pBlockFetcher NetworkBlockFetcher,
	parser BlockParser[B],
	sampler p2p.NodeSampler,
) *BlockFetcherClient[B] {
	return &BlockFetcherClient[B]{
		p2pBlockFetcher: p2pBlockFetcher,
		parser:          parser,
		sampler:         sampler,
	}
}

// FetchBlocks fetches blocks from peers **backward**.
//   - It stops when `minTS` is met (this can update dynamically, e.g., via `UpdateSyncTarget`).
//   - Each request is limited by the node's max execution time (currently ~50ms),
//     meaning multiple requests may be needed to retrieve all required blocks.
//   - If a peer is unresponsive or sends bad data, we retry with another
func (c *BlockFetcherClient[B]) FetchBlocks(ctx context.Context, blk Block, minTimestamp *atomic.Int64) <-chan B {
	resultChan := make(chan B, 1)

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
			response, err := c.p2pBlockFetcher.FetchBlocksFromPeer(reqCtx, nodeID, req)
			cancel()

			if err != nil {
				time.Sleep(backoff)
				continue
			}

			expectedParentID := c.lastBlock.GetParent()
			for _, raw := range response.Blocks {
				block, err := c.parser.ParseBlock(ctx, raw)
				if err != nil {
					break
				}

				if expectedParentID != block.GetID() {
					break
				}
				expectedParentID = block.GetParent()

				select {
				case <-ctx.Done():
					return
				case resultChan <- block:
					c.lastBlock = block
					if c.lastBlock.GetTimestamp() < minTimestamp.Load() {
						close(resultChan)
						return
					}

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
