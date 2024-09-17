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

var _ chain.Action = (*MintToken)(nil)

type MintToken struct {
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Token codec.Address `serialize:"true" json:"token"`
}

func (*MintToken) ComputeUnits(chain.Rules) uint64 {
	return MintTokenComputeUnits
}

func (m *MintToken) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) ([][]byte, error) {
	// Enforce initial invariants
	if m.Value == 0 {
		return nil, ErrOutputMintValueZero
	}
	// Check if token exists
	_, _, _, _, owner, err := storage.GetTokenInfoNoController(ctx, mu, m.Token)
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

func (*MintToken) GetTypeID() uint8 {
	return consts.MintTokenID
}

func (m *MintToken) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(m.Token)):                  state.All,
		string(storage.TokenAccountBalanceKey(m.Token, actor)): state.All,
	}
}

func (*MintToken) StateKeysMaxChunks() []uint16 {
	return []uint16{
		storage.TokenInfoChunks,
		storage.TokenAccountBalanceChunks,
	}
}

func (*MintToken) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
