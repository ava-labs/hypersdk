package dsmr

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

type Chunk[T any] struct {
	Items []T    `serialize:"true"`
	Slot  uint64 `serialize:"true"`

	bytes []byte
	id    ids.ID
}

func NewChunk[T any](items []T, slot uint64) (*Chunk[T], error) {
	c := &Chunk[T]{
		Items: items,
		Slot:  slot,
	}

	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize)}
	if err := codec.LinearCodec.MarshalInto(c, &packer); err != nil {
		return nil, err
	}
	c.bytes = packer.Bytes
	c.id = utils.ToID(c.bytes)
	return c, nil
}

type LocalChunkMempool[T any] interface {
	EstimateChunkSize() int
	StartStreaming(context.Context)
	Stream(context.Context) (T, bool)
	PrepareStreaming(context.Context)
	FinishStreaming(context.Context, []T) int
}

type Verifier[Context any, T any] interface {
	Verify(Context, T) error
}

func BuildChunk[VerifyContext any, T any](
	ctx context.Context,
	slot uint64,
	buildDuration time.Duration,
	verificationContext VerifyContext,
	verifier Verifier[VerifyContext, T],
	mempool LocalChunkMempool[T],
) (*Chunk[T], error) {
	items := make([]T, 0, mempool.EstimateChunkSize())

	start := time.Now()
	mempool.StartStreaming(ctx)
	for time.Since(start) < buildDuration {
		item, ok := mempool.Stream(ctx)
		if !ok {
			break
		}
		if err := verifier.Verify(verificationContext, item); err != nil {
			continue
		}

		items = append(items, item)
	}

	return NewChunk(items, slot)
}
