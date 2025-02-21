// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/maybe"

	"github.com/ava-labs/hypersdk/internal/typedclient"
)

const (
	requestTimeout = 1 * time.Second // Timeout for each request
	numSampleNodes = 10              // Number of nodes to sample
)

var (
	errEmptyResponse = errors.New("empty response")
	errInvalidBlock  = errors.New("invalid block")
	errChannelFull   = errors.New("result channel full")
)

// checkpoint tracks our current position, it's keeping track of the
// last valid accepted block
type checkpoint struct {
	parentID   ids.ID // last accepted parentID
	nextHeight uint64 // next block nextHeight to fetch
	timestamp  int64  // last accepted timestamp
}

type BlockParser[T Block] interface {
	ParseBlock(ctx context.Context, blockBytes []byte) (T, error)
}

// BlockFetcherClient fetches blocks from peers in a backward fashion (N, N-1, N-2, N-K) until it fills validity window of
// blocks, it ensures we have at least min validity window of blocks so we can transition from state sync to normal operation faster
type BlockFetcherClient[B Block] struct {
	client  *typedclient.TypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte]
	parser  BlockParser[B]
	sampler p2p.NodeSampler

	checkpointLock sync.RWMutex
	checkpoint     checkpoint
	once           sync.Once
}

func NewBlockFetcherClient[B Block](
	baseClient *p2p.Client,
	parser BlockParser[B],
	sampler p2p.NodeSampler,
) *BlockFetcherClient[B] {
	return &BlockFetcherClient[B]{
		client:  typedclient.NewTypedClient(baseClient, &blockFetcherMarshaler{}),
		parser:  parser,
		sampler: sampler,
	}
}

// FetchBlocks fetches blocks from peers **backward** (nextHeight N → minTS).
//   - It stops when `minTS` is met (this can update dynamically, e.g., via `UpdateSyncTarget`).
//   - Each request is limited by the node's max execution time (currently ~50ms),
//     meaning multiple requests may be needed to retrieve all required blocks.
//   - If a peer is unresponsive or sends bad data, we retry with another
func (c *BlockFetcherClient[B]) FetchBlocks(ctx context.Context, id ids.ID, height uint64, timestamp int64, minTimestamp *atomic.Int64) <-chan FetchResult[B] {
	resultChan := make(chan FetchResult[B], 100)

	// Start fetching in a separate goroutine
	go func() {
		c.checkpointLock.Lock()
		c.checkpoint = checkpoint{
			parentID:   id,
			nextHeight: height,
			timestamp:  timestamp,
		}
		c.checkpointLock.Unlock()
		req := &BlockFetchRequest{MinTimestamp: minTimestamp.Load()}

		for {
			reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)

			select {
			case <-ctx.Done():
				resultChan <- FetchResult[B]{Err: ctx.Err()}
				cancel()
				return
			default:
			}

			c.checkpointLock.RLock()
			tstamp := c.checkpoint.timestamp
			nextHeight := c.checkpoint.nextHeight
			c.checkpointLock.RUnlock()

			// Multiple blocks can share the same timestamp, so we have not filled the validity window
			// until we find and include the first block whose timestamp is strictly less than the minimum
			// timestamp. This ensures we have a complete and verifiable validity window
			if tstamp < minTimestamp.Load() {
				cancel() // Call order is important, cancel writer's before closing
				c.once.Do(func() {
					close(resultChan)
				})
				return
			}

			nodeID := c.sampleNodeID(ctx)
			if nodeID.Compare(ids.EmptyNodeID) == 0 {
				continue
			}

			req.BlockHeight = nextHeight
			err := c.client.AppRequest(reqCtx, nodeID, req, func(ctx context.Context, nodeID ids.NodeID, response *BlockFetchResponse, err error) {
				// Handle response
				if err != nil {
					resultChan <- FetchResult[B]{Err: err}
					return
				}

				respBlocks := response.Blocks
				if len(respBlocks) == 0 {
					resultChan <- FetchResult[B]{Err: fmt.Errorf("node=%s: %w", nodeID, errEmptyResponse)}
					return
				}

				c.checkpointLock.RLock()
				expectedParentID := c.checkpoint.parentID
				c.checkpointLock.RUnlock()

				for _, raw := range respBlocks {
					block, parseErr := c.parser.ParseBlock(ctx, raw)
					if parseErr != nil {
						resultChan <- FetchResult[B]{Err: fmt.Errorf("failed to parse block: %w: %w", parseErr, errInvalidBlock)}
						return
					}

					if expectedParentID != block.GetID() {
						resultChan <- FetchResult[B]{Err: fmt.Errorf("expectedParentID=%s got=%s: %w", expectedParentID, block.GetID(), errInvalidBlock)}
						return
					}
					expectedParentID = block.GetParent()

					select {
					// try to write
					case resultChan <- FetchResult[B]{Block: maybe.Some(block)}:
						// Update checkpoint
						c.checkpointLock.Lock()
						c.checkpoint.parentID = block.GetParent()
						c.checkpoint.timestamp = block.GetTimestamp()
						if nextBlkHeight := block.GetHeight(); nextBlkHeight > 0 {
							c.checkpoint.nextHeight = nextBlkHeight - 1
						}
						c.checkpointLock.Unlock()
					case <-ctx.Done():
						resultChan <- FetchResult[B]{Err: ctx.Err()}
						return
					default:
						resultChan <- FetchResult[B]{Err: errChannelFull}
						return
					}
				}
			})
			if err != nil {
				cancel()
				resultChan <- FetchResult[B]{Err: fmt.Errorf("fetch error from node=%s: %w", nodeID, err)}
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	return resultChan
}

func (c *BlockFetcherClient[B]) sampleNodeID(ctx context.Context) ids.NodeID {
	nodes := c.sampler.Sample(ctx, numSampleNodes)
	randIndex := rand.Intn(len(nodes)) //nolint:gosec
	return nodes[randIndex]
}
