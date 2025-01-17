// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package balance

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
)

const BalanceChunks uint16 = 1

var ErrInsufficientBalance = errors.New("insufficient balance")

var _ chain.BalanceHandler = (*PrefixBalanceHandler)(nil)

type PrefixBalanceHandler struct {
	Prefix []byte
}

func NewPrefixBalanceHandler(prefix []byte) *PrefixBalanceHandler {
	return &PrefixBalanceHandler{Prefix: prefix}
}

func (p *PrefixBalanceHandler) BalanceKey(addr codec.Address) []byte {
	k := make([]byte, len(p.Prefix)+codec.AddressLen+consts.Uint16Len)
	copy(k, p.Prefix)
	copy(k[len(p.Prefix):], addr[:])
	binary.BigEndian.PutUint16(k[len(p.Prefix)+codec.AddressLen:], BalanceChunks)
	return k
}

func (p *PrefixBalanceHandler) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{
		string(p.BalanceKey(addr)): state.Read | state.Write,
	}
}

func (p *PrefixBalanceHandler) CanDeduct(
	ctx context.Context,
	addr codec.Address,
	im state.Immutable,
	amount uint64,
) error {
	balance, err := p.GetBalance(ctx, addr, im)
	if err != nil {
		return err
	}
	if balance < amount {
		return fmt.Errorf("%w: %d < %d", ErrInsufficientBalance, balance, amount)
	}
	return nil
}

func (p *PrefixBalanceHandler) Deduct(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	balanceKey := p.BalanceKey(addr)
	balanceBytes, err := mu.GetValue(ctx, balanceKey)
	if err == database.ErrNotFound {
		return fmt.Errorf("%w: %d < %d", ErrInsufficientBalance, 0, amount)
	}
	if err != nil {
		return err
	}
	balance, err := database.ParseUInt64(balanceBytes)
	if err != nil {
		return err
	}
	if balance < amount {
		return fmt.Errorf("%w: %d < %d", ErrInsufficientBalance, balance, amount)
	}
	newBalance := balance - amount // No need for overflow check
	if err := mu.Insert(ctx, balanceKey, database.PackUInt64(newBalance)); err != nil {
		return fmt.Errorf("failed to deduct balance: %w", err)
	}
	return nil
}

func (p *PrefixBalanceHandler) AddBalance(
	ctx context.Context,
	addr codec.Address,
	mu state.Mutable,
	amount uint64,
) error {
	balanceKey := p.BalanceKey(addr)
	balanceBytes, err := mu.GetValue(ctx, balanceKey)
	if err != nil && err != database.ErrNotFound {
		return err
	}
	balance := uint64(0)
	if err == nil {
		balance, err = database.ParseUInt64(balanceBytes)
		if err != nil {
			return err
		}
	}

	newBalance, err := math.Add(balance, amount)
	if err != nil {
		return err
	}
	if err := mu.Insert(ctx, balanceKey, database.PackUInt64(newBalance)); err != nil {
		return fmt.Errorf("failed to add balance: %w", err)
	}
	return nil
}

func (p *PrefixBalanceHandler) GetBalance(ctx context.Context, addr codec.Address, im state.Immutable) (uint64, error) {
	balanceBytes, err := im.GetValue(ctx, p.BalanceKey(addr))
	if err == database.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	balance, err := database.ParseUInt64(balanceBytes)
	if err != nil {
		return 0, fmt.Errorf("failed to parse balance %x: %w", balanceBytes, err)
	}
	return balance, nil
}
