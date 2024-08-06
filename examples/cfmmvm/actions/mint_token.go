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

var _ chain.Action = (*MintToken)(nil)

type MintToken struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Number of assets to mint to [To].
	Value uint64 `json:"value"`

	Token codec.Address `json:"token"`
}

// ComputeUnits implements chain.Action.
func (*MintToken) ComputeUnits(chain.Rules) uint64 {
	return MintTokenComputeUnits
}

// Execute implements chain.Action.
func (m *MintToken) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Enforce initial invariants
	if m.Value == 0 {
		return nil, ErrOutputMintValueZero
	}
	// Check if token exists
	_, _, _, _, _, owner, err := storage.GetTokenInfoNoController(ctx, mu, m.Token)
	if err != nil {
		return nil, ErrOutputTokenDoesNotExist
	}
	// Check if actor is token owner
	if actor != owner {
		return nil, ErrOutputTokenNotOwner
	}

	if err := storage.MintToken(ctx, mu, m.Token, m.To, m.Value); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (*MintToken) GetTypeID() uint8 {
	return consts.MintTokenID
}

// Size implements chain.Action.
func (*MintToken) Size() int {
	return codec.AddressLen + lconsts.Uint64Len + codec.AddressLen
}

// StateKeys implements chain.Action.
func (*MintToken) StateKeys(codec.Address, ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (*MintToken) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements chain.Action.
func (*MintToken) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

// Marshal implements chain.Action.
func (m *MintToken) Marshal(p *codec.Packer) {
	p.PackAddress(m.To)
	p.PackUint64(m.Value)
	p.PackAddress(m.Token)
}

func UnmarshalMintToken(p *codec.Packer) (chain.Action, error) {
	var mintToken MintToken
	p.UnpackAddress(&mintToken.To)
	mintToken.Value = p.UnpackUint64(false)
	p.UnpackAddress(&mintToken.Token)
	return &mintToken, p.Err()
}
