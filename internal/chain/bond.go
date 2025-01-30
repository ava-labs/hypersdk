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

var _ fdsmr.Bonder[*chain.Transaction] = (*Bonder)(nil)

func NewBonder(db database.Database) Bonder {
	return Bonder{db: db}
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

	maxBalance := uint64(0)
	if len(maxBalanceBytes) > 0 {
		maxBalance = binary.BigEndian.Uint64(maxBalanceBytes)
	}

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

	batch := b.db.NewBatch()
	if err := putPendingBalance(batch, addressBytes, updatedBalance); err != nil {
		return false, err
	}

	txID := tx.GetID()
	if err := batch.Put(txID[:], binary.BigEndian.AppendUint64(nil, fee)); err != nil {
		return false, fmt.Errorf("failed to write tx fee: %w", err)
	}

	if err := batch.Write(); err != nil {
		return false, fmt.Errorf("failed to commit batch: %w", err)
	}

	return true, nil
}

func (b Bonder) Unbond(tx *chain.Transaction) error {
	address := tx.GetSponsor()
	addressBytes := address[:]

	pendingBalance, err := b.getPendingBondBalance(addressBytes)
	if err != nil {
		return err
	}

	txID := tx.GetID()
	txIDBytes := txID[:]
	feeBytes, err := b.db.Get(txIDBytes)
	if errors.Is(err, database.ErrNotFound) {
		// Make this operation idempotent if the tx was already unbonded
		// previously
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get tx fee: %w", err)
	}

	batch := b.db.NewBatch()
	fee := binary.BigEndian.Uint64(feeBytes)
	pendingBalance -= fee
	if err := putPendingBalance(batch, addressBytes, pendingBalance); err != nil {
		return err
	}

	if err := batch.Delete(txIDBytes); err != nil {
		return fmt.Errorf("failed to delete tx fee: %w", err)
	}

	if err := batch.Write(); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	return nil
}

func (b Bonder) getPendingBondBalance(address []byte) (uint64, error) {
	pendingBalanceBytes, err := b.db.Get(address)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return 0, fmt.Errorf("failed to get pending bond balance: %w", err)
	}

	if len(pendingBalanceBytes) == 0 {
		return 0, nil
	}

	return binary.BigEndian.Uint64(pendingBalanceBytes), nil
}

func putPendingBalance(batch database.Batch, address []byte, balance uint64) error {
	if err := batch.Put(address, binary.BigEndian.AppendUint64(nil, balance)); err != nil {
		return fmt.Errorf("failed to update pending bond balance: %w", err)
	}

	return nil
}

func newStateKey(b []byte) []byte {
	return keys.EncodeChunks(b, 1)
}
