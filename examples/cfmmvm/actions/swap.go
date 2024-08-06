// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/functions"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"

	lconsts "github.com/ava-labs/hypersdk/consts"
)

var _ chain.Action = (*Swap)(nil)

type Swap struct {
	AmountXIn uint64        `json:"amountXIn"`
	AmountYIn uint64        `json:"amountYIn"`
	LPAddress codec.Address `json:"lpAddress"`
}

// ComputeUnits implements chain.Action.
func (*Swap) ComputeUnits(chain.Rules) uint64 {
	return SwapComputeUnits
}

// Execute implements chain.Action.
func (s *Swap) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, reserveX, reserveY, lpTokenAddress, err := storage.GetLiquidityPoolNoController(ctx, mu, s.LPAddress)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}
	// Check that function exists
	constantFunction, err := functions.GetConstantFunction(functionID)
	if err != nil {
		return nil, ErrOutputFunctionDoesNotExist
	}
	// Call swap function
	newReserveX, newReserveY, deltaX, deltaY, err := constantFunction.Swap(reserveX, reserveY, s.AmountXIn, s.AmountYIn, fee)
	if err != nil {
		return nil, err
	}

	// Update LP
	if err := storage.SetLiquidityPool(ctx, mu, s.LPAddress, functionID, tokenX, tokenY, fee, newReserveX, newReserveY, lpTokenAddress); err != nil {
		return nil, err
	}

	// Swap tokens
	if deltaX == s.AmountXIn {
		// Take X, return Y
		if err := storage.TransferToken(ctx, mu, tokenX, actor, s.LPAddress, deltaX); err != nil {
			return nil, err
		}
		if err := storage.TransferToken(ctx, mu, tokenY, s.LPAddress, actor, deltaY); err != nil {
			return nil, err
		}
	} else {
		// Take Y, return X
		if err := storage.TransferToken(ctx, mu, tokenY, actor, s.LPAddress, deltaY); err != nil {
			return nil, err
		}
		if err := storage.TransferToken(ctx, mu, tokenX, s.LPAddress, actor, deltaX); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (*Swap) GetTypeID() uint8 {
	return consts.SwapID
}

// Size implements chain.Action.
func (*Swap) Size() int {
	return lconsts.Uint64Len + lconsts.Uint64Len
}

// StateKeys implements chain.Action.
func (*Swap) StateKeys(codec.Address, ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (*Swap) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements chain.Action.
func (*Swap) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

// Marshal implements chain.Action.
func (s *Swap) Marshal(p *codec.Packer) {
	p.PackUint64(s.AmountXIn)
	p.PackUint64(s.AmountYIn)
	p.PackAddress(s.LPAddress)
}

func UnmarshalSwap(p *codec.Packer) (chain.Action, error) {
	var swap Swap
	swap.AmountXIn = p.UnpackUint64(false)
	swap.AmountYIn = p.UnpackUint64(false)
	p.UnpackAddress(&swap.LPAddress)
	return &swap, p.Err()
}
