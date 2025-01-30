// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/neilotoole/errgroup"
	"go.uber.org/zap"
)

var isSyncing = []byte("is_syncing")

type ChainClient[T StateSummaryBlock] interface {
	LastAcceptedBlock(ctx context.Context) T
	ParseBlock(ctx context.Context, bytes []byte) (T, error)
}

type Syncer[T any] interface {
	Start(ctx context.Context, target T) error
	Wait(ctx context.Context) error
	Close() error
	UpdateSyncTarget(ctx context.Context, target T) error
}

type Client[T StateSummaryBlock] struct {
	log           logging.Logger
	chain         ChainClient[T]
	db            database.Database
	syncers       []Syncer[T]
	onStart       func(context.Context, T) error
	onFinish      func(context.Context) error
	minBlocks     uint64
	mustStateSync bool

	skipped bool
	started bool
	done    chan struct{}
	err     error
}

func NewAggregateClient[T StateSummaryBlock](
	log logging.Logger,
	chain ChainClient[T],
	db database.Database,
	syncers []Syncer[T],
	onStart func(context.Context, T) error,
	onFinish func(context.Context) error,
	minBlocks uint64,
) (*Client[T], error) {
	c := &Client[T]{
		log:       log,
		chain:     chain,
		db:        db,
		syncers:   syncers,
		minBlocks: minBlocks,
		onStart:   onStart,
		onFinish:  onFinish,
		done:      make(chan struct{}),
	}
	var err error
	c.mustStateSync, err = c.GetDiskIsSyncing()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (*Client[T]) StateSyncEnabled(context.Context) (bool, error) { return true, nil }

func (*Client[T]) GetOngoingSyncStateSummary(context.Context) (block.StateSummary, error) {
	return nil, database.ErrNotFound
}

func (c *Client[T]) ParseStateSummary(ctx context.Context, bytes []byte) (block.StateSummary, error) {
	blk, err := c.chain.ParseBlock(ctx, bytes)
	if err != nil {
		return nil, err
	}
	summary := NewSyncableBlock(blk, c)
	c.log.Info("parsed state summary", zap.Stringer("summary", summary))
	return summary, nil
}

func (c *Client[T]) Accept(
	ctx context.Context,
	target T,
) (block.StateSyncMode, error) {
	c.log.Info("Accepting state sync", zap.Stringer("target", target))
	lastAcceptedBlk := c.chain.LastAcceptedBlock(ctx)
	if !c.mustStateSync && lastAcceptedBlk.GetHeight()+c.minBlocks > target.GetHeight() {
		c.log.Info("Skipping state sync", zap.Stringer("lastAccepted", lastAcceptedBlk), zap.Stringer("target", target))
		c.skipped = true
		close(c.done)
		return block.StateSyncSkipped, nil
	}

	c.log.Info("Starting state sync", zap.Stringer("lastAccepted", lastAcceptedBlk), zap.Stringer("target", target))
	return block.StateSyncDynamic, c.startDynamicStateSync(ctx, target)
}

func (c *Client[T]) finish(ctx context.Context, err error) {
	defer func() {
		c.err = err
		close(c.done)
		// AvalancheGo does not provide a mechanism to propagate a fatal error from
		// DynamicStateSync. Log a fatal error and then panic to fail fast.
		// TODO: add a proper mechanism in AvalancheGo to propagate a fatal error
		// from the VM.
		if c.err != nil {
			c.log.Fatal("State sync failed", zap.Error(err))
			panic(c.err)
		}
	}()

	if err != nil {
		c.log.Error("state sync failed", zap.Error(err))
		return
	}

	c.log.Info("state sync completed")
	if c.onFinish != nil {
		if err := c.onFinish(ctx); err != nil {
			c.log.Error("state sync finish failed", zap.Error(err))
			return
		}
	}

	if err := c.PutDiskIsSyncing(false); err != nil {
		c.log.Error("failed to mark state sync as complete", zap.Error(err))
		return
	}
	c.log.Info("state sync finished and marked itself complete")
}

func (c *Client[T]) MustStateSync() bool {
	return c.mustStateSync
}

func (c *Client[T]) Started() bool { return c.started }

func (c *Client[T]) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("state sync timed out: %w", ctx.Err())
	case <-c.done:
		return c.err
	}
}

func (c *Client[T]) startDynamicStateSync(ctx context.Context, target T) error {
	if err := c.PutDiskIsSyncing(true); err != nil {
		return err
	}
	c.started = true

	c.log.Info("Starting state sync", zap.Stringer("target", target))
	if c.onStart != nil {
		if err := c.onStart(ctx, target); err != nil {
			c.finish(ctx, err)
			return err
		}
	}
	c.log.Info("Starting state syncer(s)", zap.Int("numSyncers", len(c.syncers)))
	detachedCtx := context.WithoutCancel(ctx)
	for _, syncer := range c.syncers {
		if err := syncer.Start(detachedCtx, target); err != nil {
			c.finish(ctx, err)
			return err
		}
	}

	c.log.Info("Kicking off state sync awaiter(s)", zap.Int("numSyncers", len(c.syncers)))
	eg, egCtx := errgroup.WithContext(detachedCtx)
	for _, syncer := range c.syncers {
		eg.Go(func() error {
			if err := syncer.Wait(egCtx); err != nil {
				return fmt.Errorf("state syncer %T failed: %w", syncer, err)
			}
			return nil
		})
	}
	go func() {
		c.log.Info("Waiting for state syncer(s) to complete", zap.Int("numSyncers", len(c.syncers)))
		err := eg.Wait()
		if err != nil {
			c.log.Fatal("State sync failed", zap.Error(err))
			panic(fmt.Sprintf("state sync failed: %s", err)) // Fail loudly
		} else {
			c.log.Info("State syncer(s) completed", zap.Int("numSyncers", len(c.syncers)))
		}
		c.finish(detachedCtx, err)
		c.log.Info("State sync completed")
	}()

	return nil
}

func (c *Client[T]) UpdateSyncTarget(ctx context.Context, target T) error {
	c.log.Info("Updating state sync target", zap.Stringer("target", target))
	for _, syncer := range c.syncers {
		if err := syncer.UpdateSyncTarget(ctx, target); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client[T]) GetDiskIsSyncing() (bool, error) {
	v, err := c.db.Get(isSyncing)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v[0] == 0x1, nil
}

func (c *Client[T]) PutDiskIsSyncing(v bool) error {
	if v {
		return c.db.Put(isSyncing, []byte{0x1})
	}
	return c.db.Put(isSyncing, []byte{0x0})
}
