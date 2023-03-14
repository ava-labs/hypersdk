// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	smath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*FillOrder)(nil)

const (
	basePrice           = 3*consts.IDLen + consts.Uint64Len + crypto.PublicKeyLen
	tradeSucceededPrice = 1_000
)

type FillOrder struct {
	// [Order] is the OrderID you wish to close.
	Order ids.ID `json:"order"`

	// [Owner] is the owner of the order and the recipient of the trade
	// proceeds.
	Owner crypto.PublicKey `json:"owner"`

	// [In] is the asset that will be sent to the owner from the fill. We need to provide this to
	// populate [StateKeys].
	In ids.ID `json:"in"`

	// [Out] is the asset that will be received from the fill. We need to provide this to
	// populate [StateKeys].
	Out ids.ID `json:"out"`

	// [Value] is the max amount of [In] that will be swapped for [Out].
	Value uint64 `json:"value"`
}

func (f *FillOrder) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	actor := auth.GetActor(rauth)
	return [][]byte{
		storage.PrefixOrderKey(f.Order),
		storage.PrefixBalanceKey(f.Owner, f.In),
		storage.PrefixBalanceKey(actor, f.In),
		storage.PrefixBalanceKey(actor, f.Out),
	}
}

func (f *FillOrder) Execute(
	ctx context.Context,
	_ chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (*chain.Result, error) {
	actor := auth.GetActor(rauth)
	exists, in, inTick, out, outTick, remaining, owner, err := storage.GetOrder(ctx, db, f.Order)
	if err != nil {
		return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
	}
	if !exists {
		return &chain.Result{Success: false, Units: basePrice, Output: OutputOrderMissing}, nil
	}
	if owner != f.Owner {
		return &chain.Result{Success: false, Units: basePrice, Output: OutputWrongOwner}, nil
	}
	if in != f.In {
		return &chain.Result{Success: false, Units: basePrice, Output: OutputWrongIn}, nil
	}
	if out != f.Out {
		return &chain.Result{Success: false, Units: basePrice, Output: OutputWrongOut}, nil
	}
	if f.Value == 0 {
		// This should be guarded via [Unmarshal] but we check anyways.
		return &chain.Result{Success: false, Units: basePrice, Output: OutputValueZero}, nil
	}
	if f.Value%inTick != 0 {
		return &chain.Result{Success: false, Units: basePrice, Output: OutputValueMisaligned}, nil
	}
	// Determine amount of [Out] counterparty will receive if the trade is
	// successful.
	outputAmount, err := smath.Mul64(outTick, f.Value/inTick)
	if err != nil {
		return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
	}
	if outputAmount == 0 {
		// This should never happen because [f.Value] > 0
		return &chain.Result{
			Success: false,
			Units:   basePrice,
			Output:  OutputInsufficientOutput,
		}, nil
	}
	var (
		inputAmount    = f.Value
		shouldDelete   = false
		orderRemaining uint64
	)
	switch {
	case outputAmount > remaining:
		// Calculate correct input given remaining supply
		//
		// This may happen if 2 people try to trade the same order at once.
		blocksOver := (outputAmount - remaining) / outTick
		inputAmount -= blocksOver * inTick

		// If the [outputAmount] is greater than remaining, take what is left.
		outputAmount = remaining
		shouldDelete = true
	case outputAmount == remaining:
		// If the [outputAmount] is equal to remaining, take all of it.
		shouldDelete = true
	default:
		orderRemaining = remaining - outputAmount
	}
	if inputAmount == 0 {
		// Don't allow free trades (can happen due to refund rounding)
		return &chain.Result{Success: false, Units: basePrice, Output: OutputInsufficientInput}, nil
	}
	if err := storage.SubBalance(ctx, db, actor, f.In, inputAmount); err != nil {
		return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, f.Owner, f.In, inputAmount); err != nil {
		return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
	}
	if err := storage.AddBalance(ctx, db, actor, f.Out, outputAmount); err != nil {
		return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
	}
	if shouldDelete {
		if err := storage.DeleteOrder(ctx, db, f.Order); err != nil {
			return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
		}
	} else {
		if err := storage.SetOrder(ctx, db, f.Order, in, inTick, out, outTick, orderRemaining, owner); err != nil {
			return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
		}
	}
	or := &OrderResult{In: inputAmount, Out: outputAmount, Remaining: orderRemaining}
	output, err := or.Marshal()
	if err != nil {
		return &chain.Result{Success: false, Units: basePrice, Output: utils.ErrBytes(err)}, nil
	}
	return &chain.Result{Success: true, Units: basePrice + tradeSucceededPrice, Output: output}, nil
}

func (*FillOrder) MaxUnits(chain.Rules) uint64 {
	return basePrice + tradeSucceededPrice
}

func (f *FillOrder) Marshal(p *codec.Packer) {
	p.PackID(f.Order)
	p.PackPublicKey(f.Owner)
	p.PackID(f.In)
	p.PackID(f.Out)
	p.PackUint64(f.Value)
}

func UnmarshalFillOrder(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var fill FillOrder
	p.UnpackID(true, &fill.Order)
	p.UnpackPublicKey(true, &fill.Owner)
	p.UnpackID(false, &fill.In)  // empty ID is the native asset
	p.UnpackID(false, &fill.Out) // empty ID is the native asset
	fill.Value = p.UnpackUint64(true)
	return &fill, p.Err()
}

func (*FillOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// OrderResult is a custom successful response output that provides information
// about a successful trade.
type OrderResult struct {
	In        uint64 `json:"in"`
	Out       uint64 `json:"out"`
	Remaining uint64 `json:"remaining"`
}

func UnmarshalOrderResult(b []byte) (*OrderResult, error) {
	p := codec.NewReader(b, consts.Uint64Len*3)
	var result OrderResult
	result.In = p.UnpackUint64(true)
	result.Out = p.UnpackUint64(true)
	result.Remaining = p.UnpackUint64(false) // if 0, deleted
	return &result, p.Err()
}

func (o *OrderResult) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.Uint64Len * 3)
	p.PackUint64(o.In)
	p.PackUint64(o.Out)
	p.PackUint64(o.Remaining)
	return p.Bytes(), p.Err()
}
