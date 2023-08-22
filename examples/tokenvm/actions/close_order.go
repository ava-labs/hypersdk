// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
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

func (c *CloseOrder) StateKeys(rauth chain.Auth, _ ids.ID) []string {
	actor := auth.GetActor(rauth)
	return []string{
		string(storage.OrderKey(c.Order)),
		string(storage.BalanceKey(actor, c.Out)),
	}
}

func (*CloseOrder) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.OrderChunks, storage.BalanceChunks}
}

func (*CloseOrder) OutputsWarpMessage() bool {
	return false
}

func (c *CloseOrder) Execute(
	ctx context.Context,
	_ chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	actor := auth.GetActor(rauth)
	exists, _, _, out, _, remaining, owner, err := storage.GetOrder(ctx, db, c.Order)
	if err != nil {
		return false, CloseOrderComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if !exists {
		return false, CloseOrderComputeUnits, OutputOrderMissing, nil, nil
	}
	if owner != actor {
		return false, CloseOrderComputeUnits, OutputUnauthorized, nil, nil
	}
	if out != c.Out {
		return false, CloseOrderComputeUnits, OutputWrongOut, nil, nil
	}
	if err := storage.DeleteOrder(ctx, db, c.Order); err != nil {
		return false, CloseOrderComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, db, actor, c.Out, remaining); err != nil {
		return false, CloseOrderComputeUnits, utils.ErrBytes(err), nil, nil
	}
	return true, CloseOrderComputeUnits, nil, nil, nil
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

func UnmarshalCloseOrder(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var cl CloseOrder
	p.UnpackID(true, &cl.Order)
	p.UnpackID(false, &cl.Out) // empty ID is the native asset
	return &cl, p.Err()
}

func (*CloseOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
