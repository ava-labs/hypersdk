// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/fdsmr"

	safemath "github.com/ava-labs/avalanchego/utils/math"
)

var (
	// ErrMissingBond is fatal and is returned if Unbond is called on a
	// transaction that was not previously bonded
	ErrMissingBond = errors.New("missing bond")

	_ fdsmr.Bonder[*chain.Transaction] = (*Bonder)(nil)
)

func NewBonder(db database.Database) Bonder {
	return Bonder{
		db: db,
	}
}

// Bonder maintains state of account bond balances to limit the amount of
// pending transactions per account
type Bonder struct{ db database.Database }

func (Bonder) SetMaxBalance(
	ctx context.Context,
	mutable state.Mutable,
	address codec.Address,
	maxBalance uint64,
) error {
	addressStateKey := newStateKey(address[:])
	if err := mutable.Insert(ctx, addressStateKey, binary.BigEndian.AppendUint64(nil, maxBalance)); err != nil {
		return fmt.Errorf("failed to update max bond balance: %w", err)
	}

	return nil
}

func (b Bonder) Bond(ctx context.Context, mutable state.Mutable, tx *chain.Transaction, feeRate uint64) (bool, error) {
	address := tx.GetSponsor()
	addressBytes := address[:]

	pendingBalance, err := b.getPendingBondBalance(addressBytes)
	if err != nil {
		return false, err
	}

	maxBalanceBytes, err := mutable.GetValue(ctx, newStateKey(addressBytes))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return false, fmt.Errorf("failed to get max bond balance: %w", err)
	}

	maxBalance := binary.BigEndian.Uint64(maxBalanceBytes)

	fee, err := safemath.Mul(uint64(tx.Size()), feeRate)
	if err != nil {
		return false, nil //nolint:nilerr
	}

	updatedBalance, err := safemath.Add(pendingBalance, fee)
	if err != nil {
		return false, nil //nolint:nilerr
	}

	if updatedBalance > maxBalance {
		return false, nil
	}

	if err := b.putPendingBalance(addressBytes, updatedBalance); err != nil {
		return false, err
	}

	txID := tx.GetID()
	if err := mutable.Insert(ctx, newStateKey(txID[:]), binary.BigEndian.AppendUint64(nil, fee)); err != nil {
		return false, fmt.Errorf("failed to write tx fee: %w", err)
	}

	return true, nil
}

func (b Bonder) Unbond(ctx context.Context, mutable state.Mutable, tx *chain.Transaction) error {
	address := tx.GetSponsor()
	addressBytes := address[:]

	pendingBalance, err := b.getPendingBondBalance(addressBytes)
	if err != nil {
		return err
	}

	txID := tx.GetID()
	txStateKey := newStateKey(txID[:])
	feeBytes, err := mutable.GetValue(ctx, txStateKey)
	if errors.Is(err, database.ErrNotFound) {
		return ErrMissingBond
	}
	if err != nil {
		return fmt.Errorf("failed to get tx fee: %w", err)
	}

	fee := binary.BigEndian.Uint64(feeBytes)
	pendingBalance -= fee
	if err := b.putPendingBalance(addressBytes, pendingBalance); err != nil {
		return err
	}

	if err := b.db.Delete(txStateKey); err != nil {
		return fmt.Errorf("failed to delete tx fee: %w", err)
	}

	return nil
}

func (b Bonder) getPendingBondBalance(address []byte) (uint64, error) {
	pendingBalanceBytes, err := b.db.Get(address)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return 0, fmt.Errorf("failed to get pending bond balance: %w", err)
	}

	return binary.BigEndian.Uint64(pendingBalanceBytes), nil
}

func (b Bonder) putPendingBalance(address []byte, balance uint64) error {
	if err := b.db.Put(address, binary.BigEndian.AppendUint64(nil, balance)); err != nil {
		return fmt.Errorf("failed to update pending bond balance: %w", err)
	}

	return nil
}

func newStateKey(b []byte) []byte {
	return keys.EncodeChunks(b, 1)
}
