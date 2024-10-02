package dsmr

import (
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

type Mempool[T Tx] interface {
	GetTxsChan() <-chan T
}

type chunkBuilder[T Tx] struct {
	threshold int
	txs       []T //TODO dedup txs?
}

// Add returns a chunk if one was built
// TODO why int64?
// TODO why error?
// TODO does not perform verification and assumes mempool is responsible for
// verifying txs
func (c *chunkBuilder[T]) Add(tx T, slot int64) (*Chunk[T], error) {
	c.txs = append(c.txs, tx)

	if len(c.txs) == c.threshold {
		chunk, err := NewChunk[T](c.txs, slot)
		c.txs = c.txs[:0]

		return chunk, err
	}

	return nil, nil
}
