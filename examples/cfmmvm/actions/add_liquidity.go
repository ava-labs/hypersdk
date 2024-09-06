// Copyright (C) 2024, Ava Labs, Inc. All rights reservea.
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

var _ chain.Action = (*AddLiquidity)(nil)

type AddLiquidity struct {
	AmountX       uint64        `json:"amountX"`
	AmountY       uint64        `json:"amountY"`
	LiquidityPool codec.Address `json:"liquidityPool"`
}

// ComputeUnits implements chain.Action.
func (*AddLiquidity) ComputeUnits(chain.Rules) uint64 {
	return AddLiquidityUnits
}

// Execute implements chain.Action.
func (a *AddLiquidity) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddress, kLast, err := storage.GetLiquidityPoolNoController(ctx, mu, a.LiquidityPool)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}

	// LP token must exist
	_, _, _, tSupply, _, err := storage.GetTokenInfoNoController(ctx, mu, lpTokenAddress)
	if err != nil {
		return nil, ErrOutputTokenDoesNotExist
	}

	initModel, ok := pricing.Models[functionID]
	if !ok {
		// TODO: rename error
		return nil, ErrOutputFunctionDoesNotExist
	}

	pricingModel := initModel(
		reserveX,
		reserveY,
		fee,
		kLast,
	)

	// TODO: add feeOwner state to liquidity pools
	tokensToActor, tokensToOwner, tokensToBurn, err := pricingModel.AddLiquidity(a.AmountX, a.AmountY, tSupply)
	if err != nil {
		return nil, err
	}

	if tokensToActor != 0 {
		if err := storage.MintToken(ctx, mu, lpTokenAddress, actor, tokensToActor); err != nil {
			return nil, err
		}
	}

	if tokensToOwner != 0 {
		if err := storage.MintToken(ctx, mu, lpTokenAddress, feeTo, tokensToOwner); err != nil {
			return nil, err
		}
	}

	if tokensToBurn != 0 {
		// Mint initial liquidity to zero address
		if err := storage.MintToken(ctx, mu, lpTokenAddress, codec.EmptyAddress, storage.MinimumLiquidity); err != nil {
			return nil, err
		}
	}

	reserveX, reserveY, k := pricingModel.GetState()

	// Update LP reserves via pricingModel
	return nil, storage.SetLiquidityPool(ctx, mu, a.LiquidityPool, functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddress, k)
}

// GetTypeID implements chain.Action.
func (*AddLiquidity) GetTypeID() uint8 {
	return consts.AddLiquidityID
}

// StateKeys implements chain.Action.
func (a *AddLiquidity) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	lpToken := storage.LiqudityPoolTokenAddress(a.LiquidityPool)
	return state.Keys{
		string(storage.LiquidityPoolKey(a.LiquidityPool)):      state.All,
		string(storage.TokenInfoKey(lpToken)):                  state.All,
		string(storage.TokenAccountBalanceKey(lpToken, actor)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (*AddLiquidity) StateKeysMaxChunks() []uint16 {
	return []uint16{
		storage.LiquidityPoolChunks,
		storage.TokenInfoChunks,
		storage.TokenAccountBalanceChunks,
	}
}

// ValidRange implements chain.Action.
func (*AddLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
