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
	_ codec.Typed  = (*RemoveLiquidityResult)(nil)
	_ chain.Action = (*RemoveLiquidity)(nil)
)

type RemoveLiquidityResult struct {
	TokenXAmount uint64 `serialize:"true" json:"tokenXAmount"`
	TokenYAmount uint64 `serialize:"true" json:"tokenYAmount"`
}

// GetTypeID implements codec.Typed.
func (r *RemoveLiquidityResult) GetTypeID() uint8 {
	return consts.RemoveLiquidityID
}

type RemoveLiquidity struct {
	BurnAmount    uint64        `serialize:"true" json:"burnAmount"`
	LiquidityPool codec.Address `serialize:"true" json:"liquidityPool"`
	TokenX        codec.Address `serialize:"true" json:"tokenX"`
	TokenY        codec.Address `serialize:"true" json:"tokenY"`
}

func (*RemoveLiquidity) ComputeUnits(chain.Rules) uint64 {
	return RemoveLiquidityUnits
}

func (l *RemoveLiquidity) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) (codec.Typed, error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddress, kLast, err := storage.GetLiquidityPoolNoController(ctx, mu, l.LiquidityPool)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}

	// Get LP token supply
	_, _, _, lpTokenAmount, _, err := storage.GetTokenInfoNoController(ctx, mu, lpTokenAddress)
	if err != nil {
		return nil, err
	}

	initModel, ok := pricing.Models[functionID]
	if !ok {
		return nil, ErrOutputFunctionDoesNotExist
	}

	pricingModel := initModel()
	pricingModel.Initialize(reserveX, reserveY, fee, kLast)

	tokensToOwner, amountX, amountY, err := pricingModel.RemoveLiquidity(l.BurnAmount, lpTokenAmount)
	if err != nil {
		return nil, err
	}

	newReserveX, newReserveY, k := pricingModel.GetState()

	// Update LP
	if err = storage.SetLiquidityPool(ctx, mu, l.LiquidityPool, functionID, tokenX, tokenY, fee, feeTo, newReserveX, newReserveY, lpTokenAddress, k); err != nil {
		return nil, err
	}

	// Send tokens to actor
	if err = storage.TransferToken(ctx, mu, tokenX, l.LiquidityPool, actor, amountX); err != nil {
		return nil, err
	}
	if err = storage.TransferToken(ctx, mu, tokenY, l.LiquidityPool, actor, amountY); err != nil {
		return nil, err
	}

	// Mint LP tokens to owner
	if err = storage.MintToken(ctx, mu, lpTokenAddress, feeTo, tokensToOwner); err != nil {
		return nil, err
	}

	// Burn LP tokens from actor
	if err = storage.BurnToken(ctx, mu, lpTokenAddress, actor, l.BurnAmount); err != nil {
		return nil, err
	}

	return &RemoveLiquidityResult{
		TokenXAmount: amountX,
		TokenYAmount: amountY,
	}, nil
}

func (*RemoveLiquidity) GetTypeID() uint8 {
	return consts.RemoveLiquidityID
}

func (l *RemoveLiquidity) StateKeys(actor codec.Address) state.Keys {
	lpTokenAddress := storage.LiqudityPoolTokenAddress(l.LiquidityPool)
	return state.Keys{
		string(storage.LiquidityPoolKey(l.LiquidityPool)):                 state.All,
		string(storage.TokenInfoKey(lpTokenAddress)):                      state.All,
		string(storage.TokenAccountBalanceKey(lpTokenAddress, actor)):     state.All,
		string(storage.TokenAccountBalanceKey(lpTokenAddress, lpAddress)): state.All,

		string(storage.TokenAccountBalanceKey(l.TokenX, actor)):     state.All,
		string(storage.TokenAccountBalanceKey(l.TokenY, actor)):     state.All,
		string(storage.TokenAccountBalanceKey(l.TokenX, lpAddress)): state.All,
		string(storage.TokenAccountBalanceKey(l.TokenY, lpAddress)): state.All,
	}
}

func (*RemoveLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
