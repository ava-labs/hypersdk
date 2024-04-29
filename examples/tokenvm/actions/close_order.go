// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
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
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (bool, uint64, []byte, error) {
	exists, _, _, out, _, remaining, owner, err := storage.GetOrder(ctx, mu, c.Order)
	if err != nil {
		return false, CloseOrderComputeUnits, utils.ErrBytes(err), nil
	}
	if !exists {
		return false, CloseOrderComputeUnits, OutputOrderMissing, nil
	}
	if owner != actor {
		return false, CloseOrderComputeUnits, OutputUnauthorized, nil
	}
	if out != c.Out {
		return false, CloseOrderComputeUnits, OutputWrongOut, nil
	}
	if err := storage.DeleteOrder(ctx, mu, c.Order); err != nil {
		return false, CloseOrderComputeUnits, utils.ErrBytes(err), nil
	}
	if err := storage.AddBalance(ctx, mu, actor, c.Out, remaining, true); err != nil {
		return false, CloseOrderComputeUnits, utils.ErrBytes(err), nil
	}
	return true, CloseOrderComputeUnits, nil, nil
}

func (*CloseOrder) MaxComputeUnits(chain.Rules) uint64 {
	return CloseOrderComputeUnits
}

func (*CloseOrder) Size() int {
	return consts.IDLen * 2
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

func (*CloseOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
