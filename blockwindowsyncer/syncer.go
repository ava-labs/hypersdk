// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/statesync"
)

var _ statesync.Syncer[Block] = (*BlockWindowSyncer[Block])(nil)

type BlockSyncer[T Block] interface {
	Accept(ctx context.Context, block T) (bool, error)
}

// BlockWindowSyncer ensures the node does not transition to normal operation
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
type BlockWindowSyncer[T Block] struct {
	forwardBlockSyncer     BlockSyncer[T]     // Handles forward syncing (new blocks)
	backwardBlockFetcher   BlockFetcher[T]    // Fetches historical blocks (backward)
	doneOnce               sync.Once          // Ensures done is signaled only once
	done                   chan struct{}      // Signals when sync is complete
	errChan                chan error         // Reports errors during syncing
	minTimestamp           atomic.Int64       // Stores the min minTimestamp needed for sync
	fetchCancel            context.CancelFunc // Cancels the fetcher when sync is done
	getTimeValidityWindowF validitywindow.GetTimeValidityWindowFunc
}

func NewBlockWindowSyncer[T Block](syncer BlockSyncer[T], fetcher BlockFetcher[T], getTimeValidityWindowF validitywindow.GetTimeValidityWindowFunc) *BlockWindowSyncer[T] {
	return &BlockWindowSyncer[T]{
		forwardBlockSyncer:     syncer,
		backwardBlockFetcher:   fetcher,
		done:                   make(chan struct{}),
		errChan:                make(chan error, 1),
		getTimeValidityWindowF: getTimeValidityWindowF,
	}
}

func (b *BlockWindowSyncer[T]) Start(ctx context.Context, target T) error {
	fetchCtx, cancel := context.WithCancel(ctx)
	b.fetchCancel = cancel
	b.minTimestamp.Store(target.GetTimestamp())

	go func() {
		err := b.backwardBlockFetcher.FetchBlocks(ctx, target, &b.minTimestamp)
		if err != nil {
			b.errChan <- fmt.Errorf("block fetcher exited with error: %w", err)
		} else {
			b.signalDone()
		}
	}()

	// aaron predlaze da forward i backward sinkere stavimo u svoj package
	// i da koristimo lock, a zapravo ono sto treba da se desi
	// jeste da backward syncer prekin egzekuciju ukoliko je skupio vise ili jednako blokova
	done, err := b.forwardBlockSyncer.Accept(fetchCtx, target)
	if err != nil {
		b.fetchCancel()
		return err
	}

	if done {
		return b.Close()
	}
	return nil
}

func (b *BlockWindowSyncer[T]) Wait(ctx context.Context) error {
	select {
	case <-b.done:
		return nil
	case err := <-b.errChan:
		return fmt.Errorf("block window syncer exited with error: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("waiting for block window syncer timed out: %w", ctx.Err())
	}
}

func (b *BlockWindowSyncer[T]) Close() error {
	b.cancelFetcher()
	b.signalDone()

	return nil
}

// UpdateSyncTarget processes a newly received target block from consensus and adjusts minTS.
//
//	Since blocks arrive dynamically,`minTS` may move forward (right), meaning:
//	 - The backward fetcher might already have retrieved enough blocks.
//	 - If so, it should be **canceled early** to avoid redundant work.
//
// Example case:
//
//	Initial state (minTS = N-3):
//	Backward Fetcher:  (Fetching history)
//	  [N-3] ← [N-2] ← [N-1] ← [N] (target)
//
//	A new target block shifts minTS one block forward (e.g., minTS = N-2):
//	Backward Fetcher: (Still fetching but minTS moved right)
//	      [N-2] ← [N-1] ← [N] ← [N+1] (target)
//
//	If the window is already filled, the fetcher should stop
func (b *BlockWindowSyncer[T]) UpdateSyncTarget(ctx context.Context, target T) error {
	done, err := b.forwardBlockSyncer.Accept(ctx, target)
	if err != nil {
		return err
	}

	if done {
		return b.Close()
	}

	b.setMinTimestamp(target.GetTimestamp())
	return nil
}

func (b *BlockWindowSyncer[T]) cancelFetcher() {
	if b.fetchCancel != nil {
		b.fetchCancel()
		b.fetchCancel = nil
	}
}

func (b *BlockWindowSyncer[T]) signalDone() {
	b.doneOnce.Do(func() {
		close(b.done)
	})
}

func (b *BlockWindowSyncer[T]) setMinTimestamp(targetTimestamp int64) {
	currentMinTS := b.minTimestamp.Load()
	newTimestamp := b.getTimeValidityWindowF(targetTimestamp - currentMinTS)
	b.minTimestamp.CompareAndSwap(currentMinTS, newTimestamp)
}
