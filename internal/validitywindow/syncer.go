// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ava-labs/hypersdk/internal/emap"
)

type BlockFetcher[T Block] interface {
	FetchBlocks(ctx context.Context, blk Block, minTimestamp *atomic.Int64) <-chan T
}

// Syncer ensures the node does not transition to normal operation
// until it has built a complete validity window of blocks.
//
// It does this using two parallel mechanisms:
//  1. Backward Fetching (historical blocks) → A goroutine fetches past blocks (N → N-K).
//  2. Forward Syncing (new blocks from consensus) → The syncer processes new incoming blocks.
//
// Forward syncing always continues to maintain an up-to-date validity window,
// even after backward fetching completes. This ensures the window stays valid
// while other components (e.g. merkle trie) finish syncing.
//
// However, if forward syncing completes the window first, it cancels backward fetching.
//
// Example timeline (`K=3` required blocks for validity window):
//
//	Backward Fetcher:  (Fetching history)
//	  [N-3] ← [N-2] ← [N-1] ← [N] (target)
//
//	Forward Syncer: (Processing new blocks from consensus)
//	  [N] → [N+1] → [N+2] → [N+3] → [N+K]
//
// The validity window can be marked as complete once either mechanism completes.
type Syncer[T emap.Item, B ExecutionBlock[T]] struct {
	chainIndex         ChainIndex[T]
	timeValidityWindow *TimeValidityWindow[T]
	getValidityWindow  GetTimeValidityWindowFunc
	blockFetcherClient BlockFetcher[B]

	oldestBlock  ExecutionBlock[T] // Tracks oldest block we have
	minTimestamp atomic.Int64      // Minimum timestamp needed for backward sync

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
	}
}

func (s *Syncer[T, B]) Start(ctx context.Context, target B) error {
	minTS := s.calculateMinTimestamp(target.GetTimestamp())
	s.minTimestamp.Store(minTS)

	// Try to build a partial validity window from existing blocks
	seenValidityWindow := s.backfillFromExisting(ctx, target)

	// If we've filled a validity window from cache/disk, we're done
	if seenValidityWindow {
		s.signalDone()
		return nil
	}

	syncCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	// Start fetching historical blocks from the peer starting from lastAccepted from cache/on-disk
	go func() {
		resultChan := s.blockFetcherClient.FetchBlocks(syncCtx, s.oldestBlock, &s.minTimestamp)
		for blk := range resultChan {
			s.timeValidityWindow.AcceptHistorical(blk)
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

func (s *Syncer[T, B]) UpdateSyncTarget(_ context.Context, target B) error {
	// Try to incorporate the new block into our window
	done := s.accept(target)
	if done {
		return s.Close()
	}

	// Update minimum timestamp based on new target
	minTS := s.calculateMinTimestamp(target.GetTimestamp())
	s.minTimestamp.Store(minTS)

	return nil
}

// accept new incoming block from consensus
func (s *Syncer[T, B]) accept(blk B) bool {
	// Check if this new block completes our validity window
	seenValidityWindow := blk.GetTimestamp()-s.oldestBlock.GetTimestamp() >
		s.getValidityWindow(blk.GetTimestamp())

	s.timeValidityWindow.Accept(blk)
	return seenValidityWindow
}

// backfillFromExisting attempts to build a validity window from existing blocks
// Returns:
// - The last accepted block (newest)
// - Whether we saw the full validity window
func (s *Syncer[T, B]) backfillFromExisting(
	ctx context.Context,
	block ExecutionBlock[T],
) bool {
	parents, seenValidityWindow := s.timeValidityWindow.PopulateValidityWindow(ctx, block)

	s.oldestBlock = parents[len(parents)-1]
	return seenValidityWindow
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
