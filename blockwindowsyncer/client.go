// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"

	"github.com/ava-labs/hypersdk/x/dsmr"
)

const (
	requestTimeout = 1 * time.Second // Timeout for each request
	numSampleNodes = 10              // Number of nodes to sample
)

var (
	errEmptyResponse = errors.New("empty response")
	errInvalidBlock  = errors.New("invalid block")
	errMaliciousNode = errors.New("malicious node")
)

type buffer[T Block] struct {
	mu      sync.Mutex
	pending map[ids.ID]T
}

func newBuffer[T Block]() *buffer[T] {
	return &buffer[T]{
		pending: make(map[ids.ID]T),
	}
}

func (b *buffer[T]) add(block T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending[block.GetID()] = block
}

func (b *buffer[T]) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = make(map[ids.ID]T)
}

func (b *buffer[T]) getAll() []T {
	b.mu.Lock()
	defer b.mu.Unlock()

	blocks := make([]T, 0, len(b.pending))
	for _, blk := range b.pending {
		blocks = append(blocks, blk)
	}
	return blocks
}

// checkpoint tracks our current position, it's useful data structure to keep track of the
// last valid written block
type checkpoint struct {
	blockID   ids.ID
	height    uint64
	timestamp int64
}

// BlockFetcherClient fetches blocks from peers in a backward fashion (N, N-1, N-2, N-K) until it fills validity window of
// blocks, it ensures we have at least min validity window of blocks so we can transition from state sync to normal operation faster
type BlockFetcherClient[T Block] struct {
	client  *dsmr.TypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte]
	parser  BlockParser[T]
	sampler p2p.NodeSampler

	buf       *buffer[T]
	writeChan chan struct{} // signals the writer goroutine to write buffered blocks
	errChan   chan error    // receives async errors (parsing/writing)

	checkpointL sync.RWMutex
	checkpoint  checkpoint

	// Stop/cleanup
	stopCh chan struct{}
	stopWG sync.WaitGroup
}

func NewBlockFetcherClient[T Block](
	baseClient *p2p.Client,
	parser BlockParser[T],
	sampler p2p.NodeSampler,
) *BlockFetcherClient[T] {
	c := &BlockFetcherClient[T]{
		client:  dsmr.NewTypedClient(baseClient, &blockFetcherMarshaler{}),
		parser:  parser,
		sampler: sampler,

		buf:       newBuffer[T](),
		writeChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		stopCh:    make(chan struct{}),
	}

	c.stopWG.Add(1)
	go func() {
		defer c.stopWG.Done()
		for {
			select {
			case <-c.stopCh:
				return
			case <-c.writeChan:
				c.writePendingBlocks(context.Background())
			}
		}
	}()

	return c
}

func (c *BlockFetcherClient[T]) Close() error {
	close(c.stopCh)
	c.stopWG.Wait()
	return nil
}

// FetchBlocks fetches blocks from peers **backward** (height N → minTS).
//   - It stops when `minTS` is met (this can update dynamically, e.g., via `UpdateSyncTarget`).
//   - Each request is limited by the node’s max execution time (currently ~50ms),
//     meaning multiple requests may be needed to retrieve all required blocks.
//   - If a peer is unresponsive or sends bad data, we retry with another
func (c *BlockFetcherClient[T]) FetchBlocks(ctx context.Context, target T, minTS *atomic.Int64) error {
	c.setCheckpoint(target.GetID(), target.GetHeight(), target.GetTimestamp())
	req := &BlockFetchRequest{MinTimestamp: minTS.Load()}

	for {
		lastCheckpoint := c.getCheckpoint()
		height := lastCheckpoint.height
		lastWrittenBlockTS := lastCheckpoint.timestamp

		if lastWrittenBlockTS <= minTS.Load() {
			break
		}

		nodeID := c.sampleNodeID(ctx)
		if nodeID.Compare(ids.EmptyNodeID) == 0 {
			// No node available
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		req.BlockHeight = height

		if err := c.client.AppRequest(reqCtx, nodeID, req, c.handleResponse); err != nil {
			cancel()
			// We'll retry with another node, so just continue
			continue
		}

		// Wait for parse/write error or context done
		select {
		case err := <-c.errChan:
			cancel()
			if errors.Is(err, context.Canceled) {
				return err
			}
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-reqCtx.Done():
			cancel()
		}
	}
	return nil
}

// handleResponse processes a peer's response
func (c *BlockFetcherClient[T]) handleResponse(ctx context.Context, _ ids.NodeID, resp *BlockFetchResponse, reqErr error) {
	if reqErr != nil {
		c.errChan <- reqErr
		return
	}
	if len(resp.Blocks) == 0 {
		c.errChan <- errEmptyResponse
		return
	}

	c.checkpointL.RLock()
	lastWritten := c.checkpoint
	c.checkpointL.RUnlock()

	expectedBlockID := lastWritten.blockID
	for _, raw := range resp.Blocks {
		blk, err := c.parser.ParseBlock(ctx, raw)
		if err != nil {
			c.errChan <- errInvalidBlock
			return
		}

		// Blocks are fetched in backward order (N -> N-1 -> N-2 -> N-K)
		// each new block must have an ID that matches the expected previous block
		// if the sequence is broken, the node is sending incorrect data
		// Valid Sequence: 	 Blk(N) -> Blk(N-1) -> Blk(N-2) -> Blk(N-3)
		// Invalid Sequence: Blk(N) -> Blk(N-1) -> Blk(X) -> Blk(X-K)
		// Blk(X) does not match the expected parent and the following blocks after Blk(X) are considered invalid
		// but, since Blk(N) and Blk(N-1) are valid, we want to write valid ones
		// and discard the rest to maximize useful data retention (goto write)
		if expectedBlockID != blk.GetID() {
			c.errChan <- errMaliciousNode
			goto write
		}
		expectedBlockID = blk.GetParent()
		c.buf.add(blk)
	}

write: // Trigger the background writer goroutine
	select {
	case c.writeChan <- struct{}{}:
	default:
	}
}

// writePendingBlocks is centralized logic for flushing the buffer and updating the checkpoint
func (c *BlockFetcherClient[T]) writePendingBlocks(ctx context.Context) {
	blocks := c.buf.getAll()
	if len(blocks) == 0 {
		return
	}

	var lastWrittenBlock T
	for _, blk := range blocks {
		if err := c.parser.WriteBlock(ctx, blk); err != nil {
			c.errChan <- fmt.Errorf("failed to write block %s: %w", blk.GetID(), err)
			c.buf.clear()
			return
		}
		lastWrittenBlock = blk
	}

	if lastWrittenBlock.GetID().Compare(ids.Empty) != 0 {
		// Since we have the last block saved and sequence matters (N, N-1, N-2, ... N-K), next block height to fetch is one below it
		c.setCheckpoint(lastWrittenBlock.GetParent(), lastWrittenBlock.GetHeight()-1, lastWrittenBlock.GetTimestamp())
	}

	// Blocks have been written successfully
	c.buf.clear()
}

func (c *BlockFetcherClient[T]) sampleNodeID(ctx context.Context) ids.NodeID {
	nodes := c.sampler.Sample(ctx, numSampleNodes)
	for _, nodeID := range nodes {
		if nodeID.Compare(ids.EmptyNodeID) != 0 {
			return nodeID
		}
	}
	return ids.EmptyNodeID
}

func (c *BlockFetcherClient[T]) setCheckpoint(blockID ids.ID, height uint64, ts int64) {
	c.checkpointL.Lock()
	defer c.checkpointL.Unlock()

	c.checkpoint.blockID = blockID
	c.checkpoint.height = height
	c.checkpoint.timestamp = ts
}

func (c *BlockFetcherClient[T]) getCheckpoint() checkpoint {
	c.checkpointL.RLock()
	defer c.checkpointL.RUnlock()

	return c.checkpoint
}
