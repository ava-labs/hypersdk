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
	_ codec.Typed  = (*GetTokenInfoResult)(nil)
	_ chain.Action = (*GetTokenInfo)(nil)
)

type GetTokenInfoResult struct {
	Name     string        `serialize:"true" json:"name"`
	Symbol   string        `serialize:"true" json:"symbol"`
	Metadata string        `serialize:"true" json:"metadata"`
	Supply   uint64        `serialize:"true" json:"supply"`
	Owner    codec.Address `serialize:"true" json:"owner"`
}

// GetTypeID implements codec.Typed.
func (*GetTokenInfoResult) GetTypeID() uint8 {
	return consts.GetTokenInfoID
}

type GetTokenInfo struct {
	Token codec.Address `serialize:"true" json:"token"`
}

func (g *GetTokenInfo) ComputeUnits(chain.Rules) uint64 {
	return GetTokenInfoUnits
}

func (g *GetTokenInfo) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
	name, symbol, metadata, supply, owner, err := storage.GetTokenInfoNoController(ctx, mu, g.Token)
	if err != nil {
		return nil, err
	}
	return &GetTokenInfoResult{
		Name:     string(name),
		Symbol:   string(symbol),
		Metadata: string(metadata),
		Supply:   supply,
		Owner:    owner,
	}, nil
}

func (g *GetTokenInfo) GetTypeID() uint8 {
	return consts.GetTokenInfoID
}

func (g *GetTokenInfo) StateKeys(actor codec.Address) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(g.Token)): state.Read,
	}
}

func (g *GetTokenInfo) ValidRange(chain.Rules) (start int64, end int64) {
	return -1, -1
}
