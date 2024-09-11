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
	Name     []byte `serialize:"true" json:"name"`
	Symbol   []byte `serialize:"true" json:"symbol"`
	Metadata []byte `serialize:"true" json:"metadata"`
}

// ComputeUnits implements chain.Action.
func (*CreateToken) ComputeUnits(chain.Rules) uint64 {
	return CreateTokenComputeUnits
}

// Execute implements chain.Action.
func (c *CreateToken) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) (outputs [][]byte, err error) {
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
	if err := storage.SetTokenInfo(ctx, mu, tokenAddress, c.Name, c.Symbol, c.Metadata, 0, actor); err != nil {
		return nil, err
	}

	// Return address
	return [][]byte{tokenAddress[:]}, nil
}

// GetTypeID implements chain.Action.
func (*CreateToken) GetTypeID() uint8 {
	return consts.CreateTokenID
}

// StateKeys implements chain.Action.
func (c *CreateToken) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(storage.TokenAddress(c.Name, c.Symbol, c.Metadata))): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (*CreateToken) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks}
}

// ValidRange implements chain.Action.
func (*CreateToken) ValidRange(chain.Rules) (start int64, end int64) {
	return -1, -1
}
