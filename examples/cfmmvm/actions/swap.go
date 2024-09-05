// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/pricing"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*Swap)(nil)

type Swap struct {
	TokenX    codec.Address `json:"tokenX"`
	TokenY    codec.Address `json:"tokenY"`
	AmountXIn uint64        `json:"amountXIn"`
	AmountYIn uint64        `json:"amountYIn"`
	LPAddress codec.Address `json:"lpAddress"`
}

// ComputeUnits implements chain.Action.
func (*Swap) ComputeUnits(chain.Rules) uint64 {
	return SwapUnits
}

// Execute implements chain.Action.
func (s *Swap) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddress, kLast, err := storage.GetLiquidityPoolNoController(ctx, mu, s.LPAddress)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}

	initModel, ok := pricing.Models[functionID]
	if !ok {
		return nil, ErrOutputFunctionDoesNotExist
	}

	pricingModel := initModel(reserveX, reserveY, fee, kLast)

	deltaX, deltaY, err := pricingModel.Swap(s.AmountXIn, s.AmountYIn)
	if err != nil {
		return nil, err
	}

	newReserveX, newReserveY, k := pricingModel.GetState()

	// Update LP
	if err := storage.SetLiquidityPool(ctx, mu, s.LPAddress, functionID, tokenX, tokenY, fee, feeTo, newReserveX, newReserveY, lpTokenAddress, k); err != nil {
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

// StateKeys implements chain.Action.
func (s *Swap) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	lpAddress := storage.LiquidityPoolAddress(s.TokenX, s.TokenY)
	return state.Keys{
		string(storage.LiquidityPoolKey(lpAddress)):             state.All,
		string(storage.TokenAccountBalanceKey(s.TokenX, actor)): state.All,
		string(storage.TokenAccountBalanceKey(s.TokenY, actor)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (*Swap) StateKeysMaxChunks() []uint16 {
	return []uint16{
		storage.LiquidityPoolChunks,
		storage.TokenAccountBalanceChunks,
		storage.TokenAccountBalanceChunks,
	}
}

// ValidRange implements chain.Action.
func (*Swap) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
