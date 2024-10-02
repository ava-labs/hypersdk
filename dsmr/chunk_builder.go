package dsmr

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
)

const (
	estimatedChunkSize             = 1024
	restorableItemsPreallocateSize = 256
)

type Tx interface {
	GetID() ids.ID
	GetExpiry() time.Time
}

type Mempool[T any] interface {
	StartStreaming(context.Context)
	Stream(context.Context, int) []T
	FinishStreaming(context.Context, []T) int
}

func BuildChunk[VerifyContext any, T any](
	ctx context.Context,
	slot int64,
	buildDuration time.Duration,
	verifier Verifier[VerifyContext, T],
	verificationContext VerifyContext,
	mempool Mempool[T],
) (*Chunk[T], error) {
	items := make([]T, 0, estimatedChunkSize)
	restorableItems := make([]T, 0, restorableItemsPreallocateSize)

	start := time.Now()
	mempool.StartStreaming(ctx)
	defer mempool.FinishStreaming(ctx, restorableItems)

	for time.Since(start) < buildDuration {
		items := mempool.Stream(ctx, 1)
		for _, item := range items {
			if err := verifier.Verify(verificationContext, item); err != nil {
				continue
			}
			items = append(items, item)
		}
	}

	return NewChunk(items, slot)
}
