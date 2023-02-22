// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*CreateOrder)(nil)

type CreateOrder struct {
	// [In] is the asset you trade for [Out].
	In ids.ID `json:"in"`

	// [Out] is the asset you receive when trading for [In].
	Out ids.ID `json:"out"`

	// [Rate] is the amount of [Out] you get per unit of [In].
	//
	// [Rate] is a decimal value stored in a uint64 (float is non-deterministic
	// depending on architecture). We divide by 10^9 when performing a fill
	// calculation.
	//
	// The output of filling an order is determined by:
	// ([Rate] * [Value (In)]) / 10^9 = [Value (Out)]
	Rate uint64 `json:"rate"`

	// [Supply] is the initial amount of [In] that the actor is locking up.
	Supply uint64 `json:"supply"`

	// Users are allowed to have any number of orders for the same [In]-[Out] pair.
}

func (c *CreateOrder) StateKeys(rauth chain.Auth, txID ids.ID) [][]byte {
	actor := auth.GetActor(rauth)
	return [][]byte{
		storage.PrefixBalanceKey(actor, c.In),
		storage.PrefixOrderKey(txID),
	}
}

func (c *CreateOrder) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	txID ids.ID,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	unitsUsed := c.MaxUnits(r) // max units == units
	if c.Rate == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputRateZero}, nil
	}
	if c.Supply == 0 {
		return &chain.Result{Success: false, Units: unitsUsed, Output: OutputSupplyZero}, nil
	}
	if err := storage.SubBalance(ctx, db, actor, c.In, c.Supply); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.SetOrder(ctx, db, txID, c.In, c.Out, c.Rate, c.Supply, actor); err != nil {
		return &chain.Result{Success: false, Units: unitsUsed, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*CreateOrder) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return consts.IDLen*2 + consts.Uint64Len*2
}

func (c *CreateOrder) Marshal(p *codec.Packer) {
	p.PackID(c.In)
	p.PackID(c.Out)
	p.PackUint64(c.Rate)
	p.PackUint64(c.Supply)
}

func UnmarshalCreateOrder(p *codec.Packer) (chain.Action, error) {
	var create CreateOrder
	p.UnpackID(false, &create.In)  // empty ID is the native asset
	p.UnpackID(false, &create.Out) // empty ID is the native asset
	create.Rate = p.UnpackUint64(true)
	create.Supply = p.UnpackUint64(true)
	return &create, p.Err()
}

func (*CreateOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
