// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/maybe"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type FetchResult[B Block] struct {
	Block maybe.Maybe[B]
	Err   error
}

type BlockFetcher[T Block] interface {
	FetchBlocks(ctx context.Context, id ids.ID, height uint64, timestamp int64, minTimestamp *atomic.Int64) <-chan FetchResult[T]
}

// Syncer ensures the node does not transition to normal operation
// until it has built a complete validity window of blocks.
//
// It does this using two parallel mechanisms:
//  1. Backward Fetching (historical blocks) → A goroutine fetches past blocks (N → N-K).
//  2. Forward Syncing (new blocks from consensus) → The syncer processes new incoming blocks.
//
// These two processes run concurrently and compete:
// - If the forward syncer completes the window first, it cancels the fetcher.
// - If the fetcher completes the window first, it stops itself.
//
// Example timeline (`K=3` required blocks for validity window):
//
//	Backward Fetcher:  (Fetching history)
//	  [N-3] ← [N-2] ← [N-1] ← [N] (target)
//
//	Forward Syncer: (Processing new blocks from consensus)
//	  [N] → [N+1] → [N+2] → [N+3] → [N+K]
//
// Whoever completes the validity window first cancels the other.
type Syncer[T emap.Item, B ExecutionBlock[T]] struct {
	chainIndex         ChainIndex[T]
	timeValidityWindow *TimeValidityWindow[T]
	getValidityWindow  GetTimeValidityWindowFunc
	blockFetcherClient BlockFetcher[B]

	lastAccepted ExecutionBlock[T] // Tracks oldest block we have
	minTimestamp *atomic.Int64     // Minimum timestamp needed for backward sync

	doneOnce sync.Once
	doneChan chan struct{}
	errChan  chan error
	cancel   context.CancelFunc // For canceling backward sync
}

func NewSyncer[T emap.Item, B ExecutionBlock[T]](chainIndex ChainIndex[T], timeValidityWindow *TimeValidityWindow[T], blockFetcherClient BlockFetcher[B], getValidityWindow GetTimeValidityWindowFunc) *Syncer[T, B] {
	return &Syncer[T, B]{
		chainIndex:         chainIndex,
		timeValidityWindow: timeValidityWindow,
		blockFetcherClient: blockFetcherClient,
		getValidityWindow:  getValidityWindow,
		doneChan:           make(chan struct{}),
		errChan:            make(chan error, 1),
		minTimestamp:       &atomic.Int64{},
	}
}

func (s *Syncer[T, B]) Start(ctx context.Context, target B) error {
	minTS := s.calculateMinTimestamp(target.GetTimestamp())
	s.minTimestamp.Store(minTS)

	// Try to build partial validity window from existing blocks
	minAccepted, seenValidityWindow := s.backfillFromExisting(ctx, target)

	// If we've filled validity window from cache/disk, we're done
	if seenValidityWindow {
		s.signalDone()
		return nil
	}

	syncCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	// Start fetching historical blocks from the peer from starting from minAccepted
	go func() {
		id := target.GetID()
		height := target.GetHeight()
		timestamp := target.GetTimestamp()

		if minAccepted != nil {
			id = minAccepted.GetID()
			height = minAccepted.GetHeight()
			timestamp = minAccepted.GetTimestamp()
		}

		resultChan := s.blockFetcherClient.FetchBlocks(syncCtx, id, height, timestamp, s.minTimestamp)
		for result := range resultChan {
			if errors.Is(result.Err, errChannelFull) || errors.Is(result.Err, context.Canceled) {
				s.cancel()
				return
			}

			if result.Block.HasValue() {
				s.timeValidityWindow.Accept(result.Block.Value())
			}
		}

		s.signalDone()
	}()

	return nil
}

func (s *Syncer[T, B]) Wait(ctx context.Context) error {
	select {
	case <-s.doneChan:
		return nil
	case err := <-s.errChan:
		return fmt.Errorf("timve valdity syncer exited with error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("waiting for time validity syncer timed out: %w", ctx.Err())
	}
}

func (s *Syncer[T, B]) Close() error {
	s.signalDone()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *Syncer[T, B]) UpdateSyncTarget(ctx context.Context, target B) error {
	// Try to incorporate the new block into our window
	done, err := s.accept(ctx, target)
	if err != nil {
		return err
	}

	if done {
		return s.Close()
	}

	// Update minimum timestamp based on new target
	minTS := s.calculateMinTimestamp(target.GetTimestamp())
	s.minTimestamp.Store(minTS)

	return nil
}

// accept new incoming block from consensus
func (s *Syncer[T, B]) accept(ctx context.Context, blk ExecutionBlock[T]) (bool, error) {
	// If we don't have any blocks yet, try to backfill
	if s.lastAccepted == nil {
		s.backfillFromExisting(ctx, blk)
	}

	// Check if this new block completes our validity window
	seenValidityWindow := blk.GetTimestamp()-s.lastAccepted.GetTimestamp() >
		s.getValidityWindow(blk.GetTimestamp())

	s.timeValidityWindow.Accept(blk)
	return seenValidityWindow, nil
}

// backfillFromExisting attempts to build validity window from existing blocks
// Returns:
// - The last block we found (oldest)
// - Whether we saw the full validity window
// - Any error encountered
func (s *Syncer[T, B]) backfillFromExisting(
	ctx context.Context,
	block ExecutionBlock[T],
) (ExecutionBlock[T], bool) {
	var (
		parent             = block
		parents            = []ExecutionBlock[T]{parent}
		seenValidityWindow = false
		validityWindow     = s.getValidityWindow(block.GetTimestamp())
		err                error
	)

	// Keep fetching parents (historical blocks) until we:
	// - Fill validity window, or
	// - Can't find more blocks
	for {
		// Get execution block from cache or disk
		parent, err = s.chainIndex.GetExecutionBlock(ctx, parent.GetParent())
		if err != nil {
			break // This is expected when we run out of cached and/or on-disk blocks
		}
		parents = append(parents, parent)

		seenValidityWindow = block.GetTimestamp()-parent.GetTimestamp() > validityWindow
		if seenValidityWindow {
			break
		}
	}

	lastIndex := len(parents) - 1
	if lastIndex < 0 {
		return nil, seenValidityWindow
	}

	minAccepted := block.GetHeight()
	minIndex := -1
	// Accept all blocks we found, even if partial
	for i := lastIndex; i >= 0; i-- {
		blk := parents[i]
		s.timeValidityWindow.Accept(blk)
		s.lastAccepted = blk

		if blk.GetHeight() < minAccepted {
			minAccepted = blk.GetHeight()
			minIndex = i
		}
	}

	if minIndex != -1 {
		return parents[minIndex], seenValidityWindow
	}
	return nil, seenValidityWindow
}

func (s *Syncer[T, B]) signalDone() {
	s.doneOnce.Do(func() {
		close(s.doneChan)
	})
}

// calculateMinTimestamp determines the oldest allowable timestamp for blocks
// in the validity window based on:
// - target block's timestamp
// - validity window duration from getValidityWindow
// The minimum timestamp is used to determine when to stop fetching historical
// blocks when backfilling the validity window.
func (s *Syncer[T, B]) calculateMinTimestamp(targetTS int64) int64 {
	validityWindow := s.getValidityWindow(targetTS)
	minTS := targetTS - validityWindow
	if minTS < 0 {
		minTS = 0
	}
	return minTS
}
