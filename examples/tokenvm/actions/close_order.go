// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*CloseOrder)(nil)

type CloseOrder struct {
	// [Order] is the OrderID you wish to close.
	Order codec.LID `json:"order"`

	// [Out] is the asset locked up in the order. We need to provide this to
	// populate [StateKeys].
	Out codec.LID `json:"out"`
}

func (*CloseOrder) GetTypeID() uint8 {
	return closeOrderID
}

func (c *CloseOrder) StateKeys(actor codec.Address, _ codec.LID) state.Keys {
	return state.Keys{
		string(storage.OrderKey(c.Order)):        state.Read | state.Write,
		string(storage.BalanceKey(actor, c.Out)): state.Read | state.Write,
	}
}

func (*CloseOrder) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.OrderChunks, storage.BalanceChunks}
}

func (c *CloseOrder) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ codec.LID,
) (bool, uint64, [][]byte) {
	exists, _, _, out, _, remaining, owner, err := storage.GetOrder(ctx, mu, c.Order)
	if err != nil {
		return false, CloseOrderComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	if !exists {
		return false, CloseOrderComputeUnits, [][]byte{OutputOrderMissing}
	}
	if owner != actor {
		return false, CloseOrderComputeUnits, [][]byte{OutputUnauthorized}
	}
	if out != c.Out {
		return false, CloseOrderComputeUnits, [][]byte{OutputWrongOut}
	}
	if err := storage.DeleteOrder(ctx, mu, c.Order); err != nil {
		return false, CloseOrderComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	if err := storage.AddBalance(ctx, mu, actor, c.Out, remaining, true); err != nil {
		return false, CloseOrderComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	return true, CloseOrderComputeUnits, [][]byte{{}}
}

func (*CloseOrder) MaxComputeUnits(chain.Rules) uint64 {
	return CloseOrderComputeUnits
}

func (*CloseOrder) Size() int {
	return codec.LIDLen * 2
}

func (c *CloseOrder) Marshal(p *codec.Packer) {
	p.PackLID(c.Order)
	p.PackLID(c.Out)
}

func UnmarshalCloseOrder(p *codec.Packer) (chain.Action, error) {
	var cl CloseOrder
	p.UnpackLID(true, &cl.Order)
	p.UnpackLID(false, &cl.Out) // empty ID is the native asset
	return &cl, p.Err()
}

func (*CloseOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
