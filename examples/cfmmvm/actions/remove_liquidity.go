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

var _ chain.Action = (*RemoveLiquidity)(nil)

type RemoveLiquidity struct {
	BurnAmount    uint64        `json:"burnAmount"`
	LiquidityPool codec.Address `json:"liquidityPool"`
}

func (*RemoveLiquidity) ComputeUnits(chain.Rules) uint64 {
	return RemoveLiquidityUnits
}

// TODO: implement fees for
// Execute implements chain.Action.
func (l *RemoveLiquidity) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
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

	pricingModel := initModel(reserveX, reserveY, fee, kLast)

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

	return nil, nil
}

// GetTypeID implements chain.Action.
func (*RemoveLiquidity) GetTypeID() uint8 {
	return consts.RemoveLiquidityID
}

// StateKeys implements chain.Action.
func (*RemoveLiquidity) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (*RemoveLiquidity) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements chain.Action.
func (*RemoveLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
