// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/x/dsmr"
)

const (
	maxRetries        = 5               // Maximum number of different peers to try
	requestTimeout    = 2 * time.Second // Fixed timeout for each request attempt
	maxSampleAttempts = 3               // Maximum attempts to sample valid nodes
)

var (
	errMaxRetriesExceeded = errors.New("max retries exceeded")
	errNoValidNodeFound   = errors.New("no valid node found")
)

// BlockFetcherClient implements fetching blocks from the network with automatic retries
// on different peers
type BlockFetcherClient[T Block] struct {
	client      *dsmr.TypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte]
	blockParser BlockParser[T]
	nodeSampler p2p.NodeSampler
	errChan     chan error // Channel to receive async errors from response handler
	parentID    ids.ID
}

func NewBlockFetcherClient[T Block](
	client *p2p.Client,
	blockParser BlockParser[T],
	nodeSampler p2p.NodeSampler,
) *BlockFetcherClient[T] {
	return &BlockFetcherClient[T]{
		client:      dsmr.NewTypedClient(client, &blockFetcherMarshaler{}),
		blockParser: blockParser,
		nodeSampler: nodeSampler,
		errChan:     make(chan error, 1),
	}
}

// FetchBlock attempts to fetch a block from the network by trying different peers
// without waiting between attempts
func (b *BlockFetcherClient[T]) FetchBlock(ctx context.Context, block T) error {
	b.parentID = block.GetParent()
	usedNodes := set.NewSet[ids.NodeID](maxRetries)
	var errs []error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during attempt %d: %w", attempt, ctx.Err())
		default:
			nodeID, err := b.sampleNodeID(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("attempt %d: failed to sample node: %w", attempt, err))
				continue
			}

			if usedNodes.Contains(nodeID) {
				continue
			}
			usedNodes.Add(nodeID)

			// Use fixed timeout for each attempt to prevent slow nodes from blocking progress
			fetchCtx, cancel := context.WithTimeout(ctx, requestTimeout)
			err = b.fetchBlockAttempt(fetchCtx, nodeID, block)
			cancel()

			if err == nil {
				return nil
			}

			// Return immediately on parent context cancellation
			if errors.Is(err, context.Canceled) {
				return fmt.Errorf("parent context cancelled during attempt with node %s: %w", nodeID, err)
			}

			// On timeout or other errors, continue to next node immediately
			errs = append(errs, fmt.Errorf("node %s: %w", nodeID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: all attempts failed after trying %d nodes: %w", errMaxRetriesExceeded, usedNodes.Len(), errors.Join(errs...))
	}

	return fmt.Errorf("no successful attempts after trying %d nodes: %w", usedNodes.Len(), errMaxRetriesExceeded)
}

// fetchBlockAttempt makes a single attempt to fetch a block from a specific node.
// It handles both the request sending and response processing through errChan.
func (b *BlockFetcherClient[T]) fetchBlockAttempt(
	ctx context.Context,
	nodeID ids.NodeID,
	block T,
) error {
	request := &BlockFetchRequest{
		BlockHeight:  block.GetHeight(),
		MinTimestamp: block.GetTimestamp(),
	}
	if err := b.client.AppRequest(ctx, nodeID, request, b.handleResponse); err != nil {
		return fmt.Errorf("failed to send request to node %s: %w", nodeID, err)
	}

	select {
	case err := <-b.errChan:
		if err != nil {
			return fmt.Errorf("error processing response from node %s: %w", nodeID, err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleResponse processes the response from a node asynchronously.
// It parses received blocks in reverse order and writes them to storage.
// Any errors during processing are sent through errChan.
func (b *BlockFetcherClient[T]) handleResponse(
	ctx context.Context,
	nodeID ids.NodeID,
	response *BlockFetchResponse,
	err error,
) {
	if err != nil {
		b.errChan <- fmt.Errorf("received error response from node %s: %w", nodeID, err)
		return
	}

	expectedParentID := b.parentID

	// Process blocks in reverse order
	lastIndex := len(response.Blocks) - 1
	for i := lastIndex; i >= 0; i-- {
		block, blkParseErr := b.blockParser.ParseBlock(ctx, response.Blocks[i])
		if blkParseErr != nil {
			b.errChan <- blkParseErr
			return
		}

		// Verify this block's parent ID matches what we expect
		if expectedParentID != block.GetID() {
			b.errChan <- fmt.Errorf("block verification failed: expected block ID %s but got %s from node %s", expectedParentID, block.GetID(), nodeID)
			// Question: Should we try another client?
			return
		}
		expectedParentID = block.GetParent()

		if blkWrtErr := b.blockParser.WriteBlock(ctx, block); blkWrtErr != nil {
			b.errChan <- blkWrtErr
			return
		}
	}

	b.errChan <- nil
}

// sampleNodeID attempts to sample a valid nodeID from the nodeSampler.
// It retries up to maxSampleAttempts times to handle temporary sampling failures
// or empty node cases.
func (b *BlockFetcherClient[T]) sampleNodeID(ctx context.Context) (ids.NodeID, error) {
	for attempt := 0; attempt < maxSampleAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ids.EmptyNodeID, ctx.Err()
		default:
			if nodes := b.nodeSampler.Sample(ctx, 1); len(nodes) > 0 {
				randIndex := rand.Intn(len(nodes))
				node := nodes[randIndex]
				if node.String() == ids.EmptyNodeID.String() {
					continue
				}
				return node, nil
			}
		}
	}

	return ids.EmptyNodeID, fmt.Errorf("failed to sample valid nodeID after %d attempts: %w",
		maxSampleAttempts, errNoValidNodeFound)
}
