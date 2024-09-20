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
)

var (
	_ codec.Typed  = (*GetLiquidityPoolInfoResult)(nil)
	_ chain.Action = (*GetLiquidityPoolInfo)(nil)
)

type GetLiquidityPoolInfoResult struct {
	TokenX         codec.Address `serialize:"true" json:"tokenX"`
	TokenY         codec.Address `serialize:"true" json:"tokenY"`
	Fee            uint64        `serialize:"true" json:"fee"`
	FeeTo          codec.Address `serialize:"true" json:"feeTo"`
	FunctionID     uint8         `serialize:"true" json:"functionID"`
	ReserveX       uint64        `serialize:"true" json:"reserveX"`
	ReserveY       uint64        `serialize:"true" json:"reserveY"`
	LiquidityToken codec.Address `serialize:"true" json:"liquidityToken"`
	KLast          uint64        `serialize:"true" json:"kLast"`
}

func (g *GetLiquidityPoolInfoResult) GetTypeID() uint8 {
	return consts.GetLiquidityPoolInfoID
}

type GetLiquidityPoolInfo struct {
	LiquidityPoolAddress codec.Address `serialize:"true" json:"liquidityPoolAddress"`
}

func (g *GetLiquidityPoolInfo) ComputeUnits(chain.Rules) uint64 {
	return GetLiquidityPoolInfoUnits
}

func (g *GetLiquidityPoolInfo) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
	functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, liquidityToken, kLast, err := storage.GetLiquidityPoolNoController(ctx, mu, g.LiquidityPoolAddress)
	if err != nil {
		return nil, err
	}
	return &GetLiquidityPoolInfoResult{
		TokenX:         tokenX,
		TokenY:         tokenY,
		Fee:            fee,
		FeeTo:          feeTo,
		FunctionID:     functionID,
		ReserveX:       reserveX,
		ReserveY:       reserveY,
		LiquidityToken: liquidityToken,
		KLast:          kLast,
	}, nil
}

func (g *GetLiquidityPoolInfo) GetTypeID() uint8 {
	return consts.GetLiquidityPoolInfoID
}

func (g *GetLiquidityPoolInfo) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.LiquidityPoolKey(g.LiquidityPoolAddress)): state.Read,
	}
}

func (g *GetLiquidityPoolInfo) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.LiquidityPoolChunks}
}

func (g *GetLiquidityPoolInfo) ValidRange(chain.Rules) (start int64, end int64) {
	return -1, -1
}
