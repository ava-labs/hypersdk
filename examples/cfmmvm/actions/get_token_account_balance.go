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
	_ codec.Typed  = (*GetTokenAccountBalanceResult)(nil)
	_ chain.Action = (*GetTokenAccountBalance)(nil)
)

type GetTokenAccountBalanceResult struct {
	Balance uint64 `serialize:"true" json:"balance"`
}

func (g *GetTokenAccountBalanceResult) GetTypeID() uint8 {
	return consts.GetTokenAccountBalanceID
}

type GetTokenAccountBalance struct {
	Token   codec.Address `serialize:"true" json:"balance"`
	Account codec.Address `serialize:"true" json:"account"`
}

func (g *GetTokenAccountBalance) ComputeUnits(chain.Rules) uint64 {
	return GetTokenAccountBalanceUnits
}

// Execute implements chain.Action.
func (g *GetTokenAccountBalance) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (codec.Typed, error) {
	balance, err := storage.GetTokenAccountBalanceNoController(ctx, mu, g.Token, g.Account)
	if err != nil {
		return nil, err
	}
	return &GetTokenAccountBalanceResult{
		Balance: balance,
	}, nil
}

// GetTypeID implements chain.Action.
func (g *GetTokenAccountBalance) GetTypeID() uint8 {
	return consts.GetTokenAccountBalanceID
}

// StateKeys implements chain.Action.
func (g *GetTokenAccountBalance) StateKeys(actor codec.Address) state.Keys {
	return state.Keys{
		string(storage.TokenAccountBalanceKey(g.Token, g.Account)): state.Read,
	}
}

// ValidRange implements chain.Action.
func (g *GetTokenAccountBalance) ValidRange(chain.Rules) (start int64, end int64) {
	return -1, -1
}
