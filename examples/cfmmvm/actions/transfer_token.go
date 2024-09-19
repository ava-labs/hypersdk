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
	_ codec.Typed  = (*TransferTokenResult)(nil)
	_ chain.Action = (*TransferToken)(nil)
)

type TransferTokenResult struct{}

func (t *TransferTokenResult) GetTypeID() uint8 {
	return consts.TransferTokenID
}

type TransferToken struct {
	To           codec.Address `serialize:"true" json:"to"`
	TokenAddress codec.Address `serialize:"true" json:"tokenAddress"`
	Value        uint64        `serialize:"true" json:"value"`
}

func (*TransferToken) ComputeUnits(chain.Rules) uint64 {
	return TransferTokenComputeUnits
}

func (t *TransferToken) Execute(ctx context.Context, _ chain.Rules, mu state.Mutable, _ int64, actor codec.Address, _ ids.ID) (codec.Typed, error) {
	// Check invariants
	if t.Value == 0 {
		return nil, ErrOutputTransferValueZero
	}

	// Check that token exists
	if _, _, _, _, _, err := storage.GetTokenInfoNoController(ctx, mu, t.TokenAddress); err != nil {
		return nil, ErrOutputTokenDoesNotExist
	}
	// Check that balance is sufficient
	balance, err := storage.GetTokenAccountBalanceNoController(ctx, mu, t.TokenAddress, actor)
	if err != nil {
		return nil, err
	}
	if balance < t.Value {
		return nil, ErrOutputInsufficientTokenBalance
	}

	if err := storage.TransferToken(ctx, mu, t.TokenAddress, actor, t.To, t.Value); err != nil {
		return nil, err
	}

	return &TransferTokenResult{}, nil
}

func (*TransferToken) GetTypeID() uint8 {
	return consts.TransferTokenID
}

func (t *TransferToken) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.TokenInfoKey(t.TokenAddress)):                  state.All,
		string(storage.TokenAccountBalanceKey(t.TokenAddress, actor)): state.All,
		string(storage.TokenAccountBalanceKey(t.TokenAddress, t.To)):  state.All,
	}
}

func (*TransferToken) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.TokenInfoChunks, storage.TokenAccountBalanceChunks, storage.TokenAccountBalanceChunks}
}

func (*TransferToken) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
