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

var _ chain.Action = (*CreateToken)(nil)

type CreateToken struct {
	Name     []byte `json:"name"`
	Symbol   []byte `json:"symbol"`
	Metadata []byte `json:"metadata"`
}

// ComputeUnits implements chain.Action.
func (*CreateToken) ComputeUnits(chain.Rules) uint64 {
	return CreateTokenComputeUnits
}

// Execute implements chain.Action.
// Returns: address of created token
func (c *CreateToken) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Enforce initial invariants
	if len(c.Name) == 0 {
		return nil, ErrOutputTokenNameEmpty
	}
	if len(c.Symbol) == 0 {
		return nil, ErrOutputTokenSymbolEmpty
	}
	if len(c.Metadata) == 0 {
		return nil, ErrOutputTokenMetadataEmpty
	}

	if len(c.Name) > storage.MaxTokenNameSize {
		return nil, ErrOutputTokenNameTooLarge
	}
	if len(c.Symbol) > storage.MaxTokenSymbolSize {
		return nil, ErrOutputTokenSymbolTooLarge
	}
	if len(c.Metadata) > storage.MaxTokenMetadataSize {
		return nil, ErrOutputTokenMetadataTooLarge
	}

	// Continue only if address doesn't exist
	tokenAddress := storage.TokenAddress(c.Name, c.Symbol, c.Metadata)
	tokenInfoKey := storage.TokenInfoKey(tokenAddress)

	if _, err := mu.GetValue(ctx, tokenInfoKey); err == nil {
		return nil, ErrOutputTokenAlreadyExists
	}

	// Invariants met; create and return
	if err := storage.SetTokenInfo(ctx, mu, tokenAddress, c.Name, c.Symbol, consts.Decimals, c.Metadata, 0, actor); err != nil {
		return nil, err
	}

	// Return address
	return [][]byte{tokenAddress[:]}, nil
}

// Size implements chain.Action.
func (c *CreateToken) Size() int {
	return codec.BytesLen(c.Name) + codec.BytesLen(c.Symbol) + codec.BytesLen(c.Metadata)
}

// ValidRange implements chain.Action.
func (*CreateToken) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (*CreateToken) GetTypeID() uint8 {
	return consts.CreateTokenID
}

func (c *CreateToken) StateKeys(codec.Address, ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(storage.TokenAddress(c.Name, c.Symbol, c.Metadata))): state.All,
	}
}

func (*CreateToken) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks}
}

// Marshal implements chain.Action.
func (c *CreateToken) Marshal(p *codec.Packer) {
	p.PackBytes(c.Name)
	p.PackBytes(c.Symbol)
	p.PackBytes(c.Metadata)
}

func UnmarhsalCreateToken(p *codec.Packer) (chain.Action, error) {
	var createToken CreateToken
	p.UnpackBytes(storage.MaxTokenNameSize, true, &createToken.Name)
	p.UnpackBytes(storage.MaxTokenSymbolSize, true, &createToken.Symbol)
	p.UnpackBytes(storage.MaxTokenMetadataSize, true, &createToken.Metadata)
	return &createToken, p.Err()
}
