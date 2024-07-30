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

	lconsts "github.com/ava-labs/hypersdk/consts"
)

var _ chain.Action = (*BurnToken)(nil)

type BurnToken struct {
	TokenAddress codec.Address `json:"tokenAddress"`
	Value        uint64        `json:"value"`
}

// ComputeUnits implements chain.Action.
func (b *BurnToken) ComputeUnits(chain.Rules) uint64 {
	return BurnTokenComputeUnits
}

// Execute implements chain.Action.
func (b *BurnToken) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Assert invariant
	if b.Value == 0 {
		return nil, ErrOutputBurnValueZero
	}

	// Check that token exists
	_, _, _, _, _, _, err := storage.GetTokenInfoNoController(ctx, mu, b.TokenAddress)
	if err != nil {
		return nil, ErrOutputTokenDoesNotExist
	}
	// Check that actor does not burn move than what they currently have
	balance, err := storage.GetTokenAccountNoController(ctx, mu, tokenOneAddress, actor)
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
func (b *BurnToken) GetTypeID() uint8 {
	return consts.BurnTokenID
}

// Size implements chain.Action.
func (b *BurnToken) Size() int {
	return codec.AddressLen + lconsts.Uint64Len
}

// StateKeys implements chain.Action.
func (b *BurnToken) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(b.TokenAddress)):           state.All,
		string(storage.TokenAccountKey(b.TokenAddress, actor)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (b *BurnToken) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks, storage.TokenAccountInfoChunks}
}

// ValidRange implements chain.Action.
func (b *BurnToken) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

// Marshal implements chain.Action.
func (b *BurnToken) Marshal(p *codec.Packer) {
	p.PackAddress(b.TokenAddress)
	p.PackUint64(b.Value)
}

func UnmarhshalBurnToken(p *codec.Packer) (chain.Action, error) {
	var burnToken BurnToken
	p.UnpackAddress(&burnToken.TokenAddress)
	burnToken.Value = p.UnpackUint64(false)
	return &burnToken, p.Err()
}
