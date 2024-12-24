// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
)

// NewGeneralMempool is creating a mempool using a FIFO queue.
type AbstractMempoolFactory[T Item] func(
	tracer trace.Tracer,
	maxSize int,
	maxSponsorSize int,
) AbstractMempool[T]

type AbstractMempool[T Item] interface {
	Has(ctx context.Context, itemID ids.ID) bool
	Add(ctx context.Context, items []T)
	PeekNext(ctx context.Context) (T, bool)
	PopNext(ctx context.Context) (T, bool)
	Remove(ctx context.Context, items []T)
	Len(ctx context.Context) int
	Size(context.Context) int
	SetMinTimestamp(ctx context.Context, t int64) []T
	Top(
		ctx context.Context,
		targetDuration time.Duration,
		f func(context.Context, T) (cont bool, restore bool, err error),
	) error
	StartStreaming(_ context.Context)
	PrepareStream(ctx context.Context, count int)
	Stream(ctx context.Context, count int) []T
	FinishStreaming(ctx context.Context, restorable []T) int
}
