// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fdsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/dsmr"
)

type Bonder[T dsmr.Tx] interface {
	// Bond returns if a transaction can be built into a chunk by this node.
	// If this returns true, Unbond is guaranteed to be called.
	Bond(tx T) bool
	// Unbond is called when a tx from an account either expires or is accepted.
	Unbond(tx T)
}

// New returns a fortified instance of DSMR
func New[T dsmr.Interface[U], U dsmr.Tx](d T, b Bonder[U]) *Node[T, U] {
	return &Node[T, U]{
		DSMR:    d,
		bonder:  b,
		pending: make(map[ids.ID]U),
	}
}

type Node[T dsmr.Interface[U], U dsmr.Tx] struct {
	DSMR   T
	bonder Bonder[U]

	pending map[ids.ID]U
}

func (n *Node[T, U]) BuildChunk(ctx context.Context, txs []U, expiry int64, beneficiary codec.Address) error {
	if len(txs) == 0 {
		return n.DSMR.BuildChunk(ctx, txs, expiry, beneficiary)
	}

	bonded := make([]U, 0, len(txs))
	for _, tx := range txs {
		if !n.bonder.Bond(tx) {
			continue
		}

		n.pending[tx.GetID()] = tx
		bonded = append(bonded, tx)
	}

	return n.DSMR.BuildChunk(ctx, bonded, expiry, beneficiary)
}

func (n *Node[T, U]) BuildBlock(parent dsmr.Block, timestamp int64) (dsmr.Block, error) {
	return n.DSMR.BuildBlock(parent, timestamp)
}

func (n *Node[T, U]) Verify(ctx context.Context, parent dsmr.Block, block dsmr.Block) error {
	return n.DSMR.Verify(ctx, parent, block)
}

func (n *Node[T, U]) Accept(ctx context.Context, block dsmr.Block) (dsmr.ExecutedBlock[U], error) {
	executedBlock, err := n.DSMR.Accept(ctx, block)
	if err != nil {
		return dsmr.ExecutedBlock[U]{}, err
	}

	// Un-bond any txs that expired at this block
	for txID, tx := range n.pending {
		if block.Timestamp <= tx.GetExpiry() {
			continue
		}

		delete(n.pending, txID)
		n.bonder.Unbond(tx)
	}

	// Un-bond any txs that were accepted in this block
	for _, chunk := range executedBlock.Chunks {
		for _, tx := range chunk.Txs {
			if _, ok := n.pending[tx.GetID()]; !ok {
				continue
			}

			n.bonder.Unbond(tx)
		}
	}

	return executedBlock, nil
}
