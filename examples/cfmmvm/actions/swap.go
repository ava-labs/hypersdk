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

var (
	_ codec.Typed  = (*SwapResult)(nil)
	_ chain.Action = (*Swap)(nil)
)

type SwapResult struct {
	AmountOut uint64        `serialize:"true" json:"amountOut"`
	TokenOut  codec.Address `serialize:"true" json:"tokenOut"`
}

func (*SwapResult) GetTypeID() uint8 {
	return consts.SwapID
}

type Swap struct {
	// First two arguments are required for `StateKeys()`
	TokenX codec.Address `serialize:"true" json:"tokenX"`
	TokenY codec.Address `serialize:"true" json:"tokenY"`

	AmountIn  uint64        `serialize:"true" json:"amountIn"`
	TokenIn   codec.Address `serialize:"true" json:"tokenIn"`
	LPAddress codec.Address `serialize:"true" json:"lpAddress"`
}

func (*Swap) ComputeUnits(chain.Rules) uint64 {
	return SwapUnits
}

func (s *Swap) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) (codec.Typed, error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddress, kLast, err := storage.GetLiquidityPoolNoController(ctx, mu, s.LPAddress)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}

	initModel, ok := pricing.Models[functionID]
	if !ok {
		return nil, ErrOutputFunctionDoesNotExist
	}

	pricingModel := initModel()
	pricingModel.Initialize(reserveX, reserveY, fee, kLast)

	swappingX := s.TokenIn == tokenX

	amountOut, err := pricingModel.Swap(s.AmountIn, swappingX)
	if err != nil {
		return nil, err
	}

	newReserveX, newReserveY, k := pricingModel.GetState()

	// Update LP
	if err := storage.SetLiquidityPool(ctx, mu, s.LPAddress, functionID, tokenX, tokenY, fee, feeTo, newReserveX, newReserveY, lpTokenAddress, k); err != nil {
		return nil, err
	}

	// Swap tokens
	if swappingX {
		// Take X, return Y
		if err := storage.TransferToken(ctx, mu, tokenX, actor, s.LPAddress, s.AmountIn); err != nil {
			return nil, err
		}
		if err := storage.TransferToken(ctx, mu, tokenY, s.LPAddress, actor, amountOut); err != nil {
			return nil, err
		}
	} else {
		// Take Y, return X
		if err := storage.TransferToken(ctx, mu, tokenY, actor, s.LPAddress, s.AmountIn); err != nil {
			return nil, err
		}
		if err := storage.TransferToken(ctx, mu, tokenX, s.LPAddress, actor, amountOut); err != nil {
			return nil, err
		}
	}

	var tokenOut codec.Address
	if swappingX {
		tokenOut = tokenY
	} else {
		tokenOut = tokenX
	}

	return &SwapResult{
		AmountOut: amountOut,
		TokenOut:  tokenOut,
	}, nil
}

func (*Swap) GetTypeID() uint8 {
	return consts.SwapID
}

func (s *Swap) StateKeys(actor codec.Address) state.Keys {
	lpAddress := storage.LiquidityPoolAddress(s.TokenX, s.TokenY)
	return state.Keys{
		string(storage.LiquidityPoolKey(lpAddress)):                 state.All,
		string(storage.TokenAccountBalanceKey(s.TokenX, actor)):     state.All,
		string(storage.TokenAccountBalanceKey(s.TokenY, actor)):     state.All,
		string(storage.TokenAccountBalanceKey(s.TokenX, lpAddress)): state.All,
		string(storage.TokenAccountBalanceKey(s.TokenY, lpAddress)): state.All,
	}
}

func (*Swap) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
