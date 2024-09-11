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

var _ chain.Action = (*BurnToken)(nil)

type BurnToken struct {
	TokenAddress codec.Address `serialize:"true" json:"tokenAddress"`
	Value        uint64        `serialize:"true" json:"value"`
}

// ComputeUnits implements chain.Action.
func (*BurnToken) ComputeUnits(chain.Rules) uint64 {
	return BurnTokenComputeUnits
}

// Execute implements chain.Action.
func (b *BurnToken) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Assert invariant
	if b.Value == 0 {
		return nil, ErrOutputBurnValueZero
	}

	// Check that token exists
	_, _, _, _, _, err := storage.GetTokenInfoNoController(ctx, mu, b.TokenAddress)
	if err != nil {
		return nil, ErrOutputTokenDoesNotExist
	}
	// Check that actor does not burn move than what they currently have
	balance, err := storage.GetTokenAccountBalanceNoController(ctx, mu, b.TokenAddress, actor)
	if err != nil {
		return nil, err
	}
	if balance < b.Value {
		return nil, ErrOutputInsufficientTokenBalance
	}

	if err := storage.BurnToken(ctx, mu, b.TokenAddress, actor, b.Value); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (*BurnToken) GetTypeID() uint8 {
	return consts.BurnTokenID
}

// StateKeys implements chain.Action.
func (b *BurnToken) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(b.TokenAddress)):                  state.All,
		string(storage.TokenAccountBalanceKey(b.TokenAddress, actor)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (*BurnToken) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks, storage.TokenAccountBalanceChunks}
}

// ValidRange implements chain.Action.
func (*BurnToken) ValidRange(chain.Rules) (start int64, end int64) {
	return -1, -1
}
