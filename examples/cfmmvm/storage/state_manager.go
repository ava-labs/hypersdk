// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ (chain.StateManager) = (*StateManager)(nil)

type StateManager struct{}

// CanDeduct implements chain.StateManager.
func (*StateManager) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
	bal, err := GetTokenAccountBalanceNoController(ctx, im, CoinAddress, addr)
	if err != nil {
		return err
	}
	if bal < amount {
		return ErrInvalidBalance
	}
	return nil
}

// Deduct implements chain.StateManager.
func (*StateManager) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	return BurnToken(ctx, mu, CoinAddress, addr, amount)
}

// AddBalance implements chain.StateManager.
func (*StateManager) AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64, _ bool) error {
	return MintToken(ctx, mu, CoinAddress, addr, amount)
}

// SponsorStateKeys implements chain.StateManager.
func (*StateManager) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(TokenAccountBalanceKey(CoinAddress, addr)): state.All,
	}
}

// FeeKey implements chain.StateManager.
func (*StateManager) FeeKey() []byte {
	return []byte{feePrefix}
}

// HeightKey implements chain.StateManager.
func (*StateManager) HeightKey() []byte {
	return []byte{heightPrefix}
}

// TimestampKey implements chain.StateManager.
func (*StateManager) TimestampKey() []byte {
	return []byte{timestampPrefix}
}
