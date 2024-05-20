// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	smath "github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*FillOrder)(nil)

type FillOrder struct {
	// [Order] is the OrderID you wish to close.
	Order codec.LID `json:"order"`

	// [Owner] is the owner of the order and the recipient of the trade
	// proceeds.
	Owner codec.Address `json:"owner"`

	// [In] is the asset that will be sent to the owner from the fill. We need to provide this to
	// populate [StateKeys].
	In codec.LID `json:"in"`

	// [Out] is the asset that will be received from the fill. We need to provide this to
	// populate [StateKeys].
	Out codec.LID `json:"out"`

	// [Value] is the max amount of [In] that will be swapped for [Out].
	Value uint64 `json:"value"`
}

func (*FillOrder) GetTypeID() uint8 {
	return fillOrderID
}

func (f *FillOrder) StateKeys(actor codec.Address, _ codec.LID) state.Keys {
	return state.Keys{
		string(storage.OrderKey(f.Order)):         state.Read | state.Write,
		string(storage.BalanceKey(f.Owner, f.In)): state.All,
		string(storage.BalanceKey(actor, f.In)):   state.All,
		string(storage.BalanceKey(actor, f.Out)):  state.All,
	}
}

func (*FillOrder) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.OrderChunks, storage.BalanceChunks, storage.BalanceChunks, storage.BalanceChunks}
}

func (f *FillOrder) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ codec.LID,
) (uint64, [][]byte, error) {
	exists, in, inTick, out, outTick, remaining, owner, err := storage.GetOrder(ctx, mu, f.Order)
	if err != nil {
		return NoFillOrderComputeUnits, nil, err
	}
	if !exists {
		return NoFillOrderComputeUnits, nil, ErrOutputOrderMissing
	}
	if owner != f.Owner {
		return NoFillOrderComputeUnits, nil, ErrOutputWrongOwner
	}
	if in != f.In {
		return NoFillOrderComputeUnits, nil, ErrOutputWrongIn
	}
	if out != f.Out {
		return NoFillOrderComputeUnits, nil, ErrOutputWrongOut
	}
	if f.Value == 0 {
		// This should be guarded via [Unmarshal] but we check anyways.
		return NoFillOrderComputeUnits, nil, ErrOutputValueZero
	}
	if f.Value%inTick != 0 {
		return NoFillOrderComputeUnits, nil, ErrOutputValueMisaligned
	}
	// Determine amount of [Out] counterparty will receive if the trade is
	// successful.
	outputAmount, err := smath.Mul64(outTick, f.Value/inTick)
	if err != nil {
		return NoFillOrderComputeUnits, nil, err
	}
	if outputAmount == 0 {
		// This should never happen because [f.Value] > 0
		return NoFillOrderComputeUnits, nil, ErrOutputInsufficientOutput
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
		return NoFillOrderComputeUnits, nil, err
	}
	if err := storage.SubBalance(ctx, mu, actor, f.In, inputAmount); err != nil {
		return NoFillOrderComputeUnits, nil, err
	}
	if err := storage.AddBalance(ctx, mu, f.Owner, f.In, inputAmount, true); err != nil {
		return NoFillOrderComputeUnits, nil, err
	}
	if err := storage.AddBalance(ctx, mu, actor, f.Out, outputAmount, true); err != nil {
		return NoFillOrderComputeUnits, nil, err
	}
	if shouldDelete {
		if err := storage.DeleteOrder(ctx, mu, f.Order); err != nil {
			return NoFillOrderComputeUnits, nil, err
		}
	} else {
		if err := storage.SetOrder(ctx, mu, f.Order, in, inTick, out, outTick, orderRemaining, owner); err != nil {
			return NoFillOrderComputeUnits, nil, err
		}
	}
	or := &OrderResult{In: inputAmount, Out: outputAmount, Remaining: orderRemaining}
	output, err := or.Marshal()
	if err != nil {
		return NoFillOrderComputeUnits, nil, err
	}
	return FillOrderComputeUnits, [][]byte{output}, nil
}

func (*FillOrder) MaxComputeUnits(chain.Rules) uint64 {
	return FillOrderComputeUnits
}

func (*FillOrder) Size() int {
	return codec.LIDLen*3 + codec.AddressLen + consts.Uint64Len
}

func (f *FillOrder) Marshal(p *codec.Packer) {
	p.PackLID(f.Order)
	p.PackLID(f.Owner)
	p.PackLID(f.In)
	p.PackLID(f.Out)
	p.PackUint64(f.Value)
}

func UnmarshalFillOrder(p *codec.Packer) (chain.Action, error) {
	var fill FillOrder
	p.UnpackLID(true, &fill.Order)
	p.UnpackLID(true, &fill.Owner)
	p.UnpackLID(false, &fill.In)  // empty ID is the native asset
	p.UnpackLID(false, &fill.Out) // empty ID is the native asset
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
	p := codec.NewWriter(consts.Uint64Len*3, consts.Uint64Len*3)
	p.PackUint64(o.In)
	p.PackUint64(o.Out)
	p.PackUint64(o.Remaining)
	return p.Bytes(), p.Err()
}
