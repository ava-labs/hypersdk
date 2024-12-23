// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"
	"errors"

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
	onStart       func(context.Context) error
	onFinish      func(context.Context) error
	minBlocks     uint64
	mustStateSync bool
}

func NewAggregateClient[T StateSummaryBlock](
	log logging.Logger,
	chain ChainClient[T],
	db database.Database,
	syncers []Syncer[T],
	onStart func(context.Context) error,
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
	}
	var err error
	c.mustStateSync, err = c.GetDiskIsSyncing()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client[T]) StateSyncEnabled(context.Context) (bool, error) { return true, nil }

func (c *Client[T]) GetOngoingSyncStateSummary(context.Context) (block.StateSummary, error) {
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
	if !c.mustStateSync && lastAcceptedBlk.Height()+c.minBlocks > target.Height() {
		c.log.Info("Skipping state sync", zap.Stringer("lastAccepted", lastAcceptedBlk), zap.Stringer("target", target))
		return block.StateSyncSkipped, nil
	}

	c.log.Info("Starting state sync", zap.Stringer("lastAccepted", lastAcceptedBlk), zap.Stringer("target", target))
	return block.StateSyncDynamic, c.startDynamicStateSync(ctx, target)
}

func (c *Client[T]) startDynamicStateSync(ctx context.Context, target T) error {
	if c.onStart != nil {
		if err := c.onStart(ctx); err != nil {
			return err
		}
	}
	for _, syncer := range c.syncers {
		if err := syncer.Start(ctx, target); err != nil {
			return err
		}
	}

	c.log.Info("Starting state syncer(s)", zap.Int("numSyncers", len(c.syncers)))
	awaitCtx := context.WithoutCancel(ctx)
	eg, egCtx := errgroup.WithContext(awaitCtx)
	for _, syncer := range c.syncers {
		eg.Go(func() error {
			return syncer.Wait(egCtx)
		})
	}
	go func() {
		c.log.Info("Waiting for state syncers to complete")
		err := eg.Wait()
		if err != nil {
			c.log.Error("state sync failed", zap.Error(err))
			panic(err)
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
	}()

	return nil
}

func (c *Client[T]) UpdateSyncTarget(ctx context.Context, target T) error {
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
