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

func (c *CloseOrder) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	actor := auth.GetActor(rauth)
	return [][]byte{
		storage.PrefixOrderKey(c.Order),
		storage.PrefixBalanceKey(actor, c.Out),
	}
}

func (c *CloseOrder) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := c.MaxUnits(r) // max units == units
	exists, _, _, out, _, remaining, owner, err := storage.GetOrder(ctx, db, c.Order)
	if err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputOrderMissing}, nil
	}
	if owner != actor {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputUnauthorized}, nil
	}
	if out != c.Out {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputWrongOut}, nil
	}
	if err := storage.DeleteOrder(ctx, db, c.Order); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, actor, c.Out, remaining); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*CloseOrder) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
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
