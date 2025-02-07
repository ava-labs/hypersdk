// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"fmt"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"sync"

	"github.com/ava-labs/hypersdk/statesync"
)

var _ statesync.Syncer[Block] = (*BlockWindowSyncer[Block])(nil)

type BlockSyncer[T Block] interface {
	Accept(ctx context.Context, block T) (bool, error)
}

// BlockWindowSyncer delays the node’s transition from state sync to normal operation
// until it has built a complete time validity window of blocks.
//
// Problem:
//   - In the standard state sync process, the validity window is filled only by processing
//     incoming blocks. If the block arrival rate is low, or if there is a backlog, the node must
//     wait until enough blocks are accepted before it can transition to normal operation.
//
// Solution:
//   - BlockWindowSyncer concurrently uses two mechanisms to build the validity window:
//     1. BlockFetcher (Client): Recursively fetches historical blocks using a dedicated p2p protocol,
//     backfilling them into the chain index. This method updates the validity window with missing
//     historical data.
//     2. BlockSyncer: Processes new (forward) blocks as they arrive, further updating the validity window.
//
// By integrating proactive historical fetching with forward block processing, this implementation
// minimizes the delay in establishing a complete validity window, thereby allowing the node to transition
// to normal operation more quickly.
type BlockWindowSyncer[T Block] struct {
	forwardBlockSyncer     BlockSyncer[T]
	backwardBlockFetcher   BlockFetcher[T]
	doneOnce               sync.Once
	done                   chan struct{}
	errChan                chan error
	fetchCancel            context.CancelFunc // cancel function for block fetcher context
	getTimeValidityWindowF validitywindow.GetTimeValidityWindowFunc
	lock                   sync.RWMutex
	lastAcceptedBlock      T
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
	// Create a cancellable context for the block fetcher.
	fetchCtx, cancel := context.WithCancel(ctx)
	b.fetchCancel = cancel

	var wg sync.WaitGroup
	go b.startBackwardBlockFetching(fetchCtx, target, &wg)

	// In the main goroutine, process forward blocks
	done, err := b.forwardBlockSyncer.Accept(ctx, target)
	if err != nil {
		// Cancel the fetcher if Accept fails
		cancel()
		return err
	}

	if done {
		// If we've filled our forward validity window, cancel the backward fetching.
		cancel()
		b.signalDone()
	} else {
		wg.Wait()
	}

	return nil
}

// Wait blocks until either the validity window is complete (done is signaled),
// an error occurs, or the provided context is cancelled.
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

// Close cancels the block fetcher if it's still running.
func (b *BlockWindowSyncer[T]) Close() error {
	b.cancelFetcher()
	return nil
}

// UpdateSyncTarget is used to update the sync target. It calls syncer.Accept with the new target,
// and if Accept returns done==true, it cancels the block fetcher and signals that we’re done.
func (b *BlockWindowSyncer[T]) UpdateSyncTarget(ctx context.Context, target T) error {
	done, err := b.forwardBlockSyncer.Accept(ctx, target)

	// timestamp of this block
	// should be taken as start block

	if err != nil {
		return err
	}

	// ovo se izgleda zove vise puta iz gorutine tokom state synca

	// pitaj Aarona - da li se oni bore ko ce napuniti validity window?
	// jer ako mi stalno prihvatamo nove blokove to znaci da cemo stalno da updejt

	b.lock.Lock()
	b.lastAcceptedBlock = target
	b.lock.Unlock()

	if done {
		b.cancelFetcher()
		b.signalDone()
	}
	return nil
}

func (b *BlockWindowSyncer[T]) startBackwardBlockFetching(ctx context.Context, target T, wg *sync.WaitGroup) {
	wg.Add(1)

	defer func() {
		wg.Done()
		if err := b.Close(); err != nil {
			select {
			case b.errChan <- err:
			default:
			}
		}
	}()

	b.lock.RLock()
	timestamp := target.GetTimestamp() - b.lastAcceptedBlock.GetTimestamp()
	b.lock.RUnlock()

	if err := b.backwardBlockFetcher.FetchBlocks(ctx, target, b.getTimeValidityWindowF(timestamp)); err != nil {
		select {
		case b.errChan <- fmt.Errorf("block fetcher error: %w", err):
		default:
		}
	}
}

// cancelFetcher cancels the block fetcher if it is still running.
func (b *BlockWindowSyncer[T]) cancelFetcher() {
	if b.fetchCancel != nil {
		b.fetchCancel()
		b.fetchCancel = nil
	}
}

// signalDone closes the done channel only once.
func (b *BlockWindowSyncer[T]) signalDone() {
	b.doneOnce.Do(func() {
		close(b.done)
	})
}
