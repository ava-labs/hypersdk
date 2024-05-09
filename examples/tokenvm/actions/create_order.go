// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*CreateOrder)(nil)

type CreateOrder struct {
	// [In] is the asset you trade for [Out].
	In codec.LID `json:"in"`

	// [InTick] is the amount of [In] required to purchase
	// [OutTick] of [Out].
	InTick uint64 `json:"inTick"`

	// [Out] is the asset you receive when trading for [In].
	//
	// This is the asset that is actually provided by the creator.
	Out codec.LID `json:"out"`

	// [OutTick] is the amount of [Out] the counterparty gets per [InTick] of
	// [In].
	OutTick uint64 `json:"outTick"`

	// [Supply] is the initial amount of [In] that the actor is locking up.
	Supply uint64 `json:"supply"`

	// Notes:
	// * Users are allowed to have any number of orders for the same [In]-[Out] pair.
	// * Using [InTick] and [OutTick] blocks ensures we avoid any odd rounding
	//	 errors.
	// * Users can fill orders with any multiple of [InTick] and will get
	//   refunded any unused assets.
}

func (*CreateOrder) GetTypeID() uint8 {
	return createOrderID
}

func (c *CreateOrder) StateKeys(actor codec.Address, actionID codec.LID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor, c.Out)): state.Read | state.Write,
		string(storage.OrderKey(actionID)):       state.Allocate | state.Write,
	}
}

func (*CreateOrder) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.OrderChunks}
}

func (c *CreateOrder) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	actionID codec.LID,
) (bool, uint64, [][]byte) {
	if c.In == c.Out {
		return false, CreateOrderComputeUnits, [][]byte{OutputSameInOut}
	}
	if c.InTick == 0 {
		return false, CreateOrderComputeUnits, [][]byte{OutputInTickZero}
	}
	if c.OutTick == 0 {
		return false, CreateOrderComputeUnits, [][]byte{OutputOutTickZero}
	}
	if c.Supply == 0 {
		return false, CreateOrderComputeUnits, [][]byte{OutputSupplyZero}
	}
	if c.Supply%c.OutTick != 0 {
		return false, CreateOrderComputeUnits, [][]byte{OutputSupplyMisaligned}
	}
	if err := storage.SubBalance(ctx, mu, actor, c.Out, c.Supply); err != nil {
		return false, CreateOrderComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	if err := storage.SetOrder(ctx, mu, actionID, c.In, c.InTick, c.Out, c.OutTick, c.Supply, actor); err != nil {
		return false, CreateOrderComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	return true, CreateOrderComputeUnits, [][]byte{{}}
}

func (*CreateOrder) MaxComputeUnits(chain.Rules) uint64 {
	return CreateOrderComputeUnits
}

func (*CreateOrder) Size() int {
	return codec.LIDLen*2 + consts.Uint64Len*3
}

func (c *CreateOrder) Marshal(p *codec.Packer) {
	p.PackLID(c.In)
	p.PackUint64(c.InTick)
	p.PackLID(c.Out)
	p.PackUint64(c.OutTick)
	p.PackUint64(c.Supply)
}

func UnmarshalCreateOrder(p *codec.Packer) (chain.Action, error) {
	var create CreateOrder
	p.UnpackLID(false, &create.In) // empty ID is the native asset
	create.InTick = p.UnpackUint64(true)
	p.UnpackLID(false, &create.Out) // empty ID is the native asset
	create.OutTick = p.UnpackUint64(true)
	create.Supply = p.UnpackUint64(true)
	return &create, p.Err()
}

func (*CreateOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func PairID(in codec.LID, out codec.LID) string {
	return fmt.Sprintf("%s-%s", in.String(), out.String())
}
