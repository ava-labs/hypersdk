// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var _ chain.Action = (*DepositLiquidity)(nil)

type DepositLiquidity struct {
	LiquidityPool codec.Address `json:"liquidityPool"`
}

// ComputeUnits implements chain.Action.
func (*DepositLiquidity) ComputeUnits(chain.Rules) uint64 {
	return DepositLiquidityComputeUnits
}

// Execute implements chain.Action.
func (d *DepositLiquidity) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) (outputs [][]byte, err error) {
	// Check that LP exists
	functionID, tokenX, tokenY, fee, reserveX, reserveY, lpTokenAddress, err := storage.GetLiquidityPoolNoController(ctx, mu, d.LiquidityPool)
	if err != nil {
		return nil, ErrOutputLiquidityPoolDoesNotExist
	}
	// TODO: implement fees
	if fee != 100 {
		return nil, ErrOutputFeeLogicNotImplemented
	}
	// LP exists, get current token balances
	balanceX, err := storage.GetTokenAccountNoController(ctx, mu, tokenX, d.LiquidityPool)
	if err != nil {
		return nil, err
	}
	balanceY, err := storage.GetTokenAccountNoController(ctx, mu, tokenY, d.LiquidityPool)
	if err != nil {
		return nil, err
	}

	_, _, _, tSupply, _, err := storage.GetTokenInfoNoController(ctx, mu, lpTokenAddress)
	if err != nil {
		return nil, ErrOutputTokenDoesNotExist
	}
	// Compute amounts
	amountX, err := smath.Sub(balanceX, reserveX)
	if err != nil {
		return nil, err
	}
	amountY, err := smath.Sub(balanceY, reserveY)
	if err != nil {
		return nil, err
	}

	var liquidity uint64
	if tSupply == 0 {
		newK, err := smath.Mul64(amountX, amountY)
		if err != nil {
			return nil, err
		}
		liquidity = sqrt(newK)
		liquidity, err = smath.Sub(liquidity, uint64(storage.MinimumLiquidity))
		if err != nil {
			return nil, err
		}
		// Mint initial liquidity to zero address
		if err := storage.MintToken(ctx, mu, lpTokenAddress, codec.EmptyAddress, uint64(storage.MinimumLiquidity)); err != nil {
			return nil, err
		}
	} else {
		tokenXChange, err := smath.Mul64(amountX, tSupply)
		if err != nil {
			return nil, err
		}
		tokenXChange /= reserveX
		tokenYChange, err := smath.Mul64(amountY, tSupply)
		if err != nil {
			return nil, err
		}
		tokenYChange /= reserveY
		liquidity = min(tokenXChange, tokenYChange)
	}

	if liquidity == 0 {
		return nil, ErrOutputInsufficientLiquidityMinted
	}

	if err := storage.MintToken(ctx, mu, lpTokenAddress, actor, liquidity); err != nil {
		return nil, err
	}

	// Update LP Reserves
	if err := storage.SetLiquidityPool(ctx, mu, d.LiquidityPool, functionID, tokenX, tokenY, fee, balanceX, balanceY, lpTokenAddress); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (*DepositLiquidity) GetTypeID() uint8 {
	return consts.DepositLiquidityID
}

// Size implements chain.Action.
func (*DepositLiquidity) Size() int {
	return codec.AddressLen
}

// StateKeys implements chain.Action.
func (*DepositLiquidity) StateKeys(codec.Address, ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (*DepositLiquidity) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements chain.Action.
func (*DepositLiquidity) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

// Marshal implements chain.Action.
func (d *DepositLiquidity) Marshal(p *codec.Packer) {
	p.PackAddress(d.LiquidityPool)
}

func UnmarhsalDepositLiquidity(p *codec.Packer) (chain.Action, error) {
	var depositLiquidity DepositLiquidity
	p.UnpackAddress(&depositLiquidity.LiquidityPool)
	return &depositLiquidity, p.Err()
}
