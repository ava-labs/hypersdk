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

func (*BalanceHandler) SponsorStateKeys(stateLayout state.Layout, addr codec.Address) state.Keys {
	return state.Keys{
		string(BalanceKey(stateLayout, addr)): state.Read | state.Write,
	}
}

func (*BalanceHandler) CanDeduct(
	ctx context.Context,
	stateLayout state.Layout,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := GetBalance(ctx, stateLayout, im, addr)
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
	stateLayout state.Layout,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	_, err := SubBalance(ctx, stateLayout, mu, addr, amount)
	return err
}

func (*BalanceHandler) AddBalance(
	ctx context.Context,
	stateLayout state.Layout,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
	createAccount bool,
) error {
	_, err := AddBalance(ctx, stateLayout, mu, addr, amount, createAccount)
	return err
}
