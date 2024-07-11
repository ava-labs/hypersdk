// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*CloseOrder)(nil)

type CloseOrder struct {
	// [Order] is the OrderID you wish to close.
	Order ids.ID `json:"order"`

	// [Out] is the asset locked up in the order. We need to provide this to
	// populate [StateKeys].
	Out ids.ID `json:"out"`
}

func (*CloseOrder) GetTypeID() uint8 {
	return closeOrderID
}

func (c *CloseOrder) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
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
	_ chain.CustomRules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	exists, _, _, out, _, remaining, owner, err := storage.GetOrder(ctx, mu, c.Order)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrOutputOrderMissing
	}
	if owner != actor {
		return nil, ErrOutputUnauthorized
	}
	if out != c.Out {
		return nil, ErrOutputWrongOut
	}
	if err := storage.DeleteOrder(ctx, mu, c.Order); err != nil {
		return nil, err
	}
	if err := storage.AddBalance(ctx, mu, actor, c.Out, remaining, true); err != nil {
		return nil, err
	}
	return nil, nil
}

func (*CloseOrder) ComputeUnits(chain.CustomRules) uint64 {
	return CloseOrderComputeUnits
}

func (*CloseOrder) Size() int {
	return ids.IDLen * 2
}

func (c *CloseOrder) Marshal(p *codec.Packer) {
	p.PackID(c.Order)
	p.PackID(c.Out)
}

func UnmarshalCloseOrder(p *codec.Packer) (chain.Action, error) {
	var cl CloseOrder
	p.UnpackID(true, &cl.Order)
	p.UnpackID(false, &cl.Out) // empty ID is the native asset
	return &cl, p.Err()
}

func (*CloseOrder) ValidRange(chain.CustomRules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
