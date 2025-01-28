// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/chain"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/fdsmr"
)

const allocSize = 16

var (
	// ErrMissingBond is fatal and is returned if Unbond is called on a
	// transaction that was not previously bonded
	ErrMissingBond = errors.New("missing bond")

	_ fdsmr.Bonder[*chain.Transaction] = (*Bonder)(nil)
)

type bondBalance struct {
	Pending *big.Int
	Max     *big.Int

	PendingBytes []byte `serialize:"true"`
	MaxBytes     []byte `serialize:"true"`
}

// Bonder maintains state of account bond balances to limit the amount of
// pending transactions per account
type Bonder struct{}

func (Bonder) SetMaxBalance(
	ctx context.Context,
	mutable state.Mutable,
	address codec.Address,
	maxBalance *big.Int,
) error {
	addressStateKey := newStateKey(address[:])
	balance, err := getBalance(ctx, mutable, addressStateKey)
	if err != nil {
		return err
	}

	balance.Max = maxBalance
	balance.MaxBytes = balance.Max.Bytes()

	if err := putBalance(ctx, mutable, addressStateKey, balance); err != nil {
		return err
	}

	return nil
}

func (Bonder) Bond(ctx context.Context, mutable state.Mutable, tx *chain.Transaction, feeRate *big.Int) (bool, error) {
	address := tx.GetSponsor()
	addressStateKey := newStateKey(address[:])

	balance, err := getBalance(ctx, mutable, addressStateKey)
	if err != nil {
		return false, err
	}

	fee := big.NewInt(int64(tx.Size()))
	fee.Mul(fee, feeRate)

	updated := big.NewInt(0).Add(balance.Pending, fee)
	if updated.Cmp(balance.Max) != -1 && fee.Cmp(big.NewInt(0)) != 0 {
		return false, nil
	}

	balance.Pending.Set(updated)
	balance.PendingBytes = balance.Pending.Bytes()
	if err := putBalance(ctx, mutable, addressStateKey, balance); err != nil {
		return false, err
	}

	if err := mutable.Insert(ctx, newStateKey(tx.id[:]), fee.Bytes()); err != nil {
		return false, fmt.Errorf("failed to write tx fee: %w", err)
	}

	return true, nil
}

func (Bonder) Unbond(ctx context.Context, mutable state.Mutable, tx *chain.Transaction) error {
	address := tx.GetSponsor()
	addressStateKey := newStateKey(address[:])

	balance, err := getBalance(ctx, mutable, addressStateKey)
	if err != nil {
		return err
	}

	txStateKey := newStateKey(tx.id[:])
	feeBytes, err := mutable.GetValue(ctx, txStateKey)
	if errors.Is(err, database.ErrNotFound) {
		return ErrMissingBond
	}
	if err != nil {
		return fmt.Errorf("failed to get tx fee: %w", err)
	}

	fee := big.NewInt(0)
	fee.SetBytes(feeBytes)

	balance.Pending.Sub(balance.Pending, fee)
	balance.PendingBytes = balance.Pending.Bytes()
	if err := putBalance(ctx, mutable, addressStateKey, balance); err != nil {
		return err
	}

	if err := mutable.Remove(ctx, txStateKey); err != nil {
		return fmt.Errorf("failed to delete tx fee: %w", err)
	}

	return nil
}

func getBalance(ctx context.Context, mutable state.Mutable, address []byte) (bondBalance, error) {
	currentBytes, err := mutable.GetValue(ctx, address)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return bondBalance{}, fmt.Errorf("failed to get bond balance: %w", err)
	}

	if currentBytes == nil {
		currentBytes = make([]byte, allocSize)
	}

	balance := bondBalance{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: currentBytes},
		&balance,
	); err != nil {
		return bondBalance{}, fmt.Errorf("failed to unmarshal bond balance: %w", err)
	}

	balance.Pending = big.NewInt(0).SetBytes(balance.PendingBytes)
	balance.Max = big.NewInt(0).SetBytes(balance.MaxBytes)
	return balance, nil
}

func putBalance(ctx context.Context, mutable state.Mutable, address []byte, balance bondBalance) error {
	p := &wrappers.Packer{Bytes: make([]byte, allocSize)}
	if err := codec.LinearCodec.MarshalInto(balance, p); err != nil {
		return fmt.Errorf("failed to marshal bond balance: %w", err)
	}

	if err := mutable.Insert(ctx, address, p.Bytes); err != nil {
		return fmt.Errorf("failed to update bond balance: %w", err)
	}

	return nil
}

func newStateKey(b []byte) []byte {
	return keys.EncodeChunks(b, 1)
}
