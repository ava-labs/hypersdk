// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package statesync

import (
	"context"
	"fmt"
	"sync"
)

var _ Syncer[interface{}] = (*BlockWindowSyncer[interface{}])(nil)

type BlockSyncer[T any] interface {
	Accept(ctx context.Context, block T) (bool, error)
}

type BlockWindowSyncer[T any] struct {
	syncer   BlockSyncer[T]
	doneOnce sync.Once
	done     chan struct{}
}

func NewBlockWindowSyncer[T any](syncer BlockSyncer[T]) *BlockWindowSyncer[T] {
	return &BlockWindowSyncer[T]{
		syncer: syncer,
		done:   make(chan struct{}),
	}
}

func (b *BlockWindowSyncer[T]) Start(ctx context.Context, target T) error {
	done, err := b.syncer.Accept(ctx, target)
	if done {
		b.doneOnce.Do(func() {
			close(b.done)
		})
	}
	return err
}

func (b *BlockWindowSyncer[T]) Wait(ctx context.Context) error {
	select {
	case <-b.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("failed to await full block window: %w", ctx.Err())
	}
}

func (*BlockWindowSyncer[T]) Close() error {
	return nil
}

func (b *BlockWindowSyncer[T]) UpdateSyncTarget(ctx context.Context, target T) error {
	done, err := b.syncer.Accept(ctx, target)
	if done {
		b.doneOnce.Do(func() {
			close(b.done)
		})
	}
	return err
}
