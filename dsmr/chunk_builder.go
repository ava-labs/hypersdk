// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
)

type Tx interface {
	GetID() ids.ID
	GetExpiry() time.Time
}

type chunkBuilder[T Tx] struct {
	threshold int
	txs       []T // TODO dedup txs?
}

// Add returns a chunk if one was built
// TODO why int64?
// TODO why error?
// TODO does not perform verification and assumes mempool is responsible for
// verifying tx
func (c *chunkBuilder[T]) Add(tx T, expiry int64) (Chunk[T], error) {
	c.txs = append(c.txs, tx)

	if len(c.txs) == c.threshold {
		chunk, err := NewChunk[T](c.txs, expiry)
		c.txs = c.txs[:0]

		return chunk, err
	}

	return Chunk[T]{}, nil
}
