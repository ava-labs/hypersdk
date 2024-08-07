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

var _ chain.Action = (*CreateLiquidityPool)(nil)

type CreateLiquidityPool struct {
	FunctionID uint64        `json:"functionID"`
	TokenX     codec.Address `json:"tokenX"`
	TokenY     codec.Address `json:"tokenY"`
	Fee        uint64        `json:"fee"`
}

// ComputeUnits implements chain.Action.
func (*CreateLiquidityPool) ComputeUnits(chain.Rules) uint64 {
	return CreateLiquidityPoolComputeUnits
}

// Execute implements chain.Action.
// Outputs: liqudity pool address and liqudity pool token address
func (c *CreateLiquidityPool) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, _ codec.Address, _ ids.ID) ([][]byte, error) {
	// Assert argument invariants
	if !isValidFee(c.Fee) {
		return nil, ErrOutputInvalidFee
	}
	// Check that tokens exist
	_, _, _, _, _, err := storage.GetTokenInfoNoController(ctx, mu, c.TokenX)
	if err != nil {
		return nil, ErrOutputTokenXDoesNotExist
	}
	_, _, _, _, _, err = storage.GetTokenInfoNoController(ctx, mu, c.TokenY)
	if err != nil {
		return nil, ErrOutputTokenYDoesNotExist
	}
	if !isValidConstantFunction(c.FunctionID) {
		return nil, ErrOutputFunctionDoesNotExist
	}

	poolAddress, err := storage.LiquidityPoolAddress(c.TokenX, c.TokenY)
	if err != nil {
		return nil, ErrOutputIdenticalTokens
	}
	// Check that LP does not exist
	_, _, _, _, _, _, _, err = storage.GetLiquidityPoolNoController(ctx, mu, poolAddress)
	if err == nil {
		return nil, ErrOutputLiquidityPoolAlreadyExists
	}
	// Create token
	lpTokenAddress := storage.LiqudityPoolTokenAddress(poolAddress)
	if err := storage.SetTokenInfo(ctx, mu, lpTokenAddress, []byte(storage.LiquidityPoolTokenName), []byte(storage.LiquidityPoolTokenSymbol), []byte(storage.LiquidityPoolTokenMetadata), 0, poolAddress); err != nil {
		return nil, err
	}
	// Create LP
	if err = storage.SetLiquidityPool(ctx, mu, poolAddress, c.FunctionID, c.TokenX, c.TokenY, c.Fee, 0, 0, lpTokenAddress); err != nil {
		return nil, err
	}

	return [][]byte{poolAddress[:], lpTokenAddress[:]}, nil
}

// GetTypeID implements chain.Action.
func (*CreateLiquidityPool) GetTypeID() uint8 {
	return consts.CreateLiquidityPoolID
}

// Size implements chain.Action.
func (*CreateLiquidityPool) Size() int {
	return lconsts.Uint64Len + codec.AddressLen + codec.AddressLen + lconsts.Uint64Len
}

// StateKeys implements chain.Action.
func (c *CreateLiquidityPool) StateKeys(codec.Address, ids.ID) state.Keys {
	tokenXKey := storage.TokenInfoKey(c.TokenX)
	tokenYKey := storage.TokenInfoKey(c.TokenY)
	// TODO: address err value returned here
	lpAddress, _ := storage.LiquidityPoolAddress(c.TokenX, c.TokenY)
	lpKey := storage.LiquidityPoolKey(lpAddress)
	lpTokenKey := storage.TokenInfoKey(storage.LiqudityPoolTokenAddress(lpAddress))
	return state.Keys{
		string(tokenXKey):  state.Read,
		string(tokenYKey):  state.Read,
		string(lpKey):      state.All,
		string(lpTokenKey): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (*CreateLiquidityPool) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks, storage.TokenInfoChunks, storage.LiquidityPoolChunks, storage.TokenInfoChunks}
}

// ValidRange implements chain.Action.
func (*CreateLiquidityPool) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

// Marshal implements chain.Action.
func (c *CreateLiquidityPool) Marshal(p *codec.Packer) {
	p.PackUint64(c.FunctionID)
	p.PackAddress(c.TokenX)
	p.PackAddress(c.TokenY)
	p.PackUint64(c.Fee)
}

func UnmarshalCreateLiquidityPool(p *codec.Packer) (chain.Action, error) {
	var createLiquidityPool CreateLiquidityPool
	createLiquidityPool.Fee = p.UnpackUint64(false)
	p.UnpackAddress(&createLiquidityPool.TokenX)
	p.UnpackAddress(&createLiquidityPool.TokenY)
	createLiquidityPool.FunctionID = p.UnpackUint64(false)
	return &createLiquidityPool, p.Err()
}

func isValidFee(fee uint64) bool {
	return fee > 0 && fee <= 100
}

func isValidConstantFunction(functionID uint64) bool {
	if _, err := functions.GetConstantFunction(functionID); err != nil {
		return false
	}
	return true
}
