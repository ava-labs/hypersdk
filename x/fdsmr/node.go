// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fdsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/dsmr"
)

type Interface[T dsmr.Tx] interface {
	BuildChunk(ctx context.Context, txs []T, expiry int64, beneficiary codec.Address) error
	Accept(ctx context.Context, block dsmr.Block) (dsmr.ExecutedBlock[T], error)
}

type Bonder[T dsmr.Tx] interface {
	// Bond returns if a transaction can be built into a chunk by this node.
	// If this returns true, Unbond is guaranteed to be called.
	Bond(tx T) (bool, error)
	// Unbond is called when a tx from an account either expires or is accepted.
	// If Unbond is called, Bond is guaranteed to have been called previously on
	// tx.
	Unbond(tx T) error
}

// New returns a fortified instance of DSMR
func New[T Interface[U], U dsmr.Tx](inner T, bonder Bonder[U]) *Node[T, U] {
	return &Node[T, U]{
		DSMR:    inner,
		bonder:  bonder,
		pending: make(map[ids.ID]U),
	}
}

type Node[T Interface[U], U dsmr.Tx] struct {
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
		ok, err := n.bonder.Bond(tx)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		n.pending[tx.GetID()] = tx
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
	for txID, tx := range n.pending {
		if block.Timestamp <= tx.GetExpiry() {
			continue
		}

		if err := n.bonder.Unbond(tx); err != nil {
			return dsmr.ExecutedBlock[U]{}, nil
		}
		delete(n.pending, txID)
	}

	// Un-bond any txs that were accepted in this block
	for _, chunk := range executedBlock.Chunks {
		for _, tx := range chunk.Txs {
			if _, ok := n.pending[tx.GetID()]; !ok {
				continue
			}

			if err := n.bonder.Unbond(tx); err != nil {
				return dsmr.ExecutedBlock[U]{}, err
			}
		}
	}

	return executedBlock, nil
}
