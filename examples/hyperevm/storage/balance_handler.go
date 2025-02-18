// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.BalanceHandler = (*BalanceHandler)(nil)

type BalanceHandler struct{}

func (b *BalanceHandler) AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	_, err := AddBalance(ctx, mu, ToEVMAddress(addr), amount)
	return err
}

func (b *BalanceHandler) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
	bal, err := GetBalance(ctx, im, ToEVMAddress(addr))
	if err != nil {
		return err
	}
	if bal < amount {
		return fmt.Errorf("%w: cannot deduct (balance=%d, addr=%v, amount=%d)",
			ErrInvalidBalance,
			bal,
			addr,
			amount,
		)
	}
	return nil
}

func (b *BalanceHandler) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	_, err := SubBalance(ctx, mu, ToEVMAddress(addr), amount)
	return err
}

func (b *BalanceHandler) GetBalance(ctx context.Context, addr codec.Address, im state.Immutable) (uint64, error) {
	return GetBalance(ctx, im, ToEVMAddress(addr))
}

func (b *BalanceHandler) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(AccountKey(ToEVMAddress(addr))): state.Read | state.Write,
	}
}
