// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ (chain.BalanceHandler) = (*BalanceHandler)(nil)

type BalanceHandler struct{}

func (*BalanceHandler) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(BalanceKey(addr)): state.Read | state.Write,
	}
}

func (*BalanceHandler) CanDeduct(
	ctx context.Context,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := GetBalance(ctx, im, addr)
	if err != nil {
		return err
	}
	if bal < amount {
		return ErrInvalidBalance
	}
	return nil
}

func (*BalanceHandler) Deduct(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	_, err := SubBalance(ctx, mu, addr, amount)
	return err
}

func (*BalanceHandler) AddBalance(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	_, err := AddBalance(ctx, mu, addr, amount)
	return err
}

func (*BalanceHandler) GetBalance(ctx context.Context, addr codec.Address, im state.Immutable) (uint64, error) {
	return GetBalance(ctx, im, addr)
}
