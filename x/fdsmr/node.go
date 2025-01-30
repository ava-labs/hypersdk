// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fdsmr

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/eheap"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/dsmr"
)

type DSMR[T dsmr.Tx] interface {
	BuildChunk(ctx context.Context, txs []T, expiry int64, beneficiary codec.Address) error
	Accept(ctx context.Context, block dsmr.Block) (dsmr.ExecutedBlock[T], error)
}

type Bonder[T dsmr.Tx] interface {
	// Bond returns if a transaction can be built into a chunk by this node.
	// If this returns true, Unbond is guaranteed to be called.
	Bond(ctx context.Context, mutable state.Mutable, tx T, fee uint64) (bool, error)
	// Unbond is called when a tx from an account either expires or is accepted.
	// If Unbond is called, Bond is guaranteed to have been called previously on
	// tx.
	Unbond(tx T) error
}

// New returns a fortified instance of DSMR
func New[T DSMR[U], U dsmr.Tx](inner T, bonder Bonder[U]) *Node[T, U] {
	return &Node[T, U]{
		DSMR:    inner,
		bonder:  bonder,
		pending: eheap.New[U](0),
	}
}

type Node[T DSMR[U], U dsmr.Tx] struct {
	DSMR   T
	bonder Bonder[U]

	pending *eheap.ExpiryHeap[U]
}

func (n *Node[T, U]) BuildChunk(ctx context.Context, mutable state.Mutable, txs []U, expiry int64, beneficiary codec.Address, fee uint64) error {
	bonded := make([]U, 0, len(txs))
	for _, tx := range txs {
		ok, err := n.bonder.Bond(ctx, mutable, tx, fee)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		n.pending.Add(tx)
		bonded = append(bonded, tx)
	}

	return n.DSMR.BuildChunk(ctx, bonded, expiry, beneficiary)
}

func (n *Node[T, U]) Accept(ctx context.Context, block dsmr.Block) (dsmr.ExecutedBlock[U], error) {
	executedBlock, err := n.DSMR.Accept(ctx, block)
	if err != nil {
		return dsmr.ExecutedBlock[U]{}, err
	}

	// Un-bond any txs that expired at this block
	expired := n.pending.SetMin(block.Timestamp)
	for _, tx := range expired {
		if err := n.bonder.Unbond(tx); err != nil {
			return dsmr.ExecutedBlock[U]{}, err
		}
	}

	// Un-bond any txs that were accepted in this block
	for _, chunk := range executedBlock.Chunks {
		for _, tx := range chunk.Txs {
			if !n.pending.Has(tx.GetID()) {
				continue
			}

			if err := n.bonder.Unbond(tx); err != nil {
				return dsmr.ExecutedBlock[U]{}, err
			}
		}
	}

	return executedBlock, nil
}
