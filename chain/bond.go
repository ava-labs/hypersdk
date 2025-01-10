// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/fdsmr"
)

const bondAllocSize = 128

var (
	// ErrMissingBond is fatal and is returned if Unbond is called on a
	// transaction that was not previously bonded
	ErrMissingBond = errors.New("missing bond")

	_ fdsmr.Bonder[*Transaction] = (*Bonder)(nil)
)

type BondBalance struct {
	Pending uint32 `serialize:"true"`
	Max     uint32 `serialize:"true"`
}

// Bonder maintains state of account bond balances to limit the amount of
// pending transactions per account
type Bonder struct{}

// this needs to be thread-safe if it's called from the api
func (Bonder) SetMaxBalance(
	ctx context.Context,
	mutable state.Mutable,
	address codec.Address,
	maxBalance uint32,
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

func (Bonder) Bond(ctx context.Context, mutable state.Mutable, tx *Transaction) (bool, error) {
	address := tx.GetSponsor()
	addressBytes := address[:]

	balance, err := getBalance(ctx, mutable, addressBytes)
	if err != nil {
		return false, err
	}

	if balance.Pending >= balance.Max {
		return false, nil
	}

	balance.Pending++
	if err := putBalance(ctx, mutable, addressBytes, balance); err != nil {
		return false, err
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

	if balance.Pending == 0 {
		return ErrMissingBond
	}

	balance.Pending--
	if err := putBalance(ctx, mutable, addressBytes, balance); err != nil {
		return err
	}

	return nil
}

func getBalance(ctx context.Context, mutable state.Mutable, address []byte) (BondBalance, error) {
	currentBytes, err := mutable.GetValue(ctx, address)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return BondBalance{}, fmt.Errorf("failed to get bond balance: %w", err)
	}

	if currentBytes == nil {
		currentBytes = make([]byte, bondAllocSize)
	}

	balance := BondBalance{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: currentBytes},
		&balance,
	); err != nil {
		return BondBalance{}, fmt.Errorf("failed to unmarshal bond balance: %w", err)
	}

	return balance, nil
}

func putBalance(ctx context.Context, mutable state.Mutable, address []byte, balance BondBalance) error {
	p := &wrappers.Packer{Bytes: make([]byte, bondAllocSize)}
	if err := codec.LinearCodec.MarshalInto(balance, p); err != nil {
		return fmt.Errorf("failed to marshal bond balance: %w", err)
	}

	if err := mutable.Insert(ctx, address, p.Bytes); err != nil {
		return fmt.Errorf("failed to update bond balance: %w", err)
	}

	return nil
}
