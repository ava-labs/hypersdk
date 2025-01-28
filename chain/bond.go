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

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/fdsmr"
)

var (
	// ErrMissingBond is fatal and is returned if Unbond is called on a
	// transaction that was not previously bonded
	ErrMissingBond = errors.New("missing bond")

	_ fdsmr.Bonder[*Transaction] = (*Bonder)(nil)
)

type BondBalance struct {
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
	addressBytes := address[:]

	balance, err := getBalance(ctx, mutable, addressBytes)
	if err != nil {
		return err
	}

	balance.Max = maxBalance
	if err := putBalance(ctx, mutable, addressBytes, balance); err != nil {
		return err
	}

	return nil
}

func (Bonder) Bond(ctx context.Context, mutable state.Mutable, tx *Transaction, feeRate *big.Int) (bool, error) {
	address := tx.GetSponsor()
	addressBytes := address[:]

	balance, err := getBalance(ctx, mutable, addressBytes)
	if err != nil {
		return false, err
	}

	if balance.Pending.Cmp(balance.Max) != -1 {
		return false, nil
	}

	fee := big.NewInt(int64(tx.Size()))
	fee.Mul(fee, feeRate)

	balance.Pending.Sub(balance.Pending, fee)
	if err := putBalance(ctx, mutable, addressBytes, balance); err != nil {
		return false, err
	}

	if err := mutable.Insert(ctx, tx.id[:], fee.Bytes()); err != nil {
		return false, fmt.Errorf("failed to write tx fee: %w", err)
	}

	return true, nil
}

func (Bonder) Unbond(ctx context.Context, mutable state.Mutable, tx *Transaction) error {
	address := tx.GetSponsor()
	addressBytes := address[:]

	balance, err := getBalance(ctx, mutable, addressBytes)
	if err != nil {
		return err
	}

	if balance.Pending.Cmp(big.NewInt(0)) == 0 {
		return ErrMissingBond
	}

	feeBytes, err := mutable.GetValue(ctx, tx.id[:])
	if err != nil {
		return fmt.Errorf("failed to get tx fee: %w", err)
	}

	fee := big.NewInt(0)
	fee.SetBytes(feeBytes)

	balance.Pending.Add(balance.Pending, fee)
	if err := putBalance(ctx, mutable, addressBytes, balance); err != nil {
		return err
	}

	if err := mutable.Remove(ctx, tx.id[:]); err != nil {
		return fmt.Errorf("failed to delete tx fee: %w", err)
	}

	return nil
}

func getBalance(ctx context.Context, mutable state.Mutable, address []byte) (BondBalance, error) {
	currentBytes, err := mutable.GetValue(ctx, address)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return BondBalance{}, fmt.Errorf("failed to get bond balance: %w", err)
	}

	if currentBytes == nil {
		currentBytes = make([]byte, 0)
	}

	balance := BondBalance{
		//PendingBytes: []byte{},
		//MaxBytes:     []byte{},
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: currentBytes},
		&balance,
	); err != nil {
		return BondBalance{}, fmt.Errorf("failed to unmarshal bond balance: %w", err)
	}

	balance.Pending = big.NewInt(0).SetBytes(balance.PendingBytes)
	balance.Max = big.NewInt(0).SetBytes(balance.MaxBytes)
	return balance, nil
}

func putBalance(ctx context.Context, mutable state.Mutable, address []byte, balance BondBalance) error {
	p := &wrappers.Packer{Bytes: make([]byte, 0)}
	if err := codec.LinearCodec.MarshalInto(balance, p); err != nil {
		return fmt.Errorf("failed to marshal bond balance: %w", err)
	}

	if err := mutable.Insert(ctx, address, p.Bytes); err != nil {
		return fmt.Errorf("failed to update bond balance: %w", err)
	}

	return nil
}
