// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package defaultvm

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/utils"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var _ chain.TransactionExecutor = (*DefaultTransactionExecutor)(nil)

type DefaultTransactionExecutor struct{}

func (d DefaultTransactionExecutor) Run(
	ctx context.Context,
	tx *chain.Transaction,
	stateKeys state.Keys,
	storage map[string][]byte,
	feeManager *internalfees.Manager,
	bh chain.BalanceHandler,
	r chain.Rules,
	ts *tstate.TState,
	timestamp int64,
	blockHeight uint64,
) (*chain.Result, error) {
	tsv := ts.NewView(stateKeys, storage)

	if err := d.PreExecute(ctx, tx, feeManager, bh, r, tsv, timestamp); err != nil {
		return nil, err
	}

	result, err := d.Execute(ctx, tx, feeManager, bh, r, tsv, timestamp)
	if err != nil {
		return nil, err
	}

	// Commit results to parent [TState]
	tsv.Commit()

	return result, nil
}

func (DefaultTransactionExecutor) PreExecute(
	ctx context.Context,
	tx *chain.Transaction,
	feeManager *internalfees.Manager,
	bh chain.BalanceHandler,
	r chain.Rules,
	im state.Immutable,
	timestamp int64,
) error {
	if err := tx.Base.Execute(r, timestamp); err != nil {
		return err
	}
	if len(tx.Actions) > int(r.GetMaxActionsPerTx()) {
		return chain.ErrTooManyActions
	}
	for i, action := range tx.Actions {
		start, end := action.ValidRange(r)
		if start >= 0 && timestamp < start {
			return fmt.Errorf("%w: action type %d at index %d", chain.ErrActionNotActivated, action.GetTypeID(), i)
		}
		if end >= 0 && timestamp > end {
			return fmt.Errorf("%w: action type %d at index %d", chain.ErrActionNotActivated, action.GetTypeID(), i)
		}
	}
	start, end := tx.Auth.ValidRange(r)
	if start >= 0 && timestamp < start {
		return chain.ErrAuthNotActivated
	}
	if end >= 0 && timestamp > end {
		return chain.ErrAuthNotActivated
	}
	units, err := tx.Units(bh, r)
	if err != nil {
		return err
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		return err
	}
	return bh.CanDeduct(ctx, tx.Auth.Sponsor(), im, fee)
}

// Execute after knowing a transaction can pay a fee. Attempt
// to charge the fee in as many cases as possible.
//
// Invariant: [PreExecute] is called just before [Execute]
func (DefaultTransactionExecutor) Execute(
	ctx context.Context,
	tx *chain.Transaction,
	feeManager *internalfees.Manager,
	bh chain.BalanceHandler,
	r chain.Rules,
	ts *tstate.TStateView,
	timestamp int64,
) (*chain.Result, error) {
	// Always charge fee first
	units, err := tx.Units(bh, r)
	if err != nil {
		// Should never happen
		return nil, fmt.Errorf("failed to calculate tx units: %w", err)
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		// Should never happen
		return nil, fmt.Errorf("failed to calculate tx fee: %w", err)
	}
	if err := bh.Deduct(ctx, tx.Auth.Sponsor(), ts, fee); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before).
		return nil, fmt.Errorf("failed to deduct tx fee: %w", err)
	}

	// We create a temp state checkpoint to ensure we don't commit failed actions to state.
	//
	// We should favor reverting over returning an error because the caller won't be charged
	// for a transaction that returns an error.
	var (
		actionStart   = ts.OpIndex()
		actionOutputs = [][]byte{}
	)
	for i, action := range tx.Actions {
		actionOutput, err := action.Execute(ctx, r, ts, timestamp, tx.Auth.Actor(), chain.CreateActionID(tx.GetID(), uint8(i)))
		if err != nil {
			ts.Rollback(ctx, actionStart)
			return &chain.Result{
				Success: false,
				Error:   utils.ErrBytes(err),
				Outputs: actionOutputs,
				Units:   units,
				Fee:     fee,
			}, nil
		}

		var encodedOutput []byte
		if actionOutput == nil {
			// Ensure output standardization (match form we will
			// unmarshal)
			encodedOutput = []byte{}
		} else {
			encodedOutput, err = chain.MarshalTyped(actionOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal action output %T: %w", actionOutput, err)
			}
		}

		actionOutputs = append(actionOutputs, encodedOutput)
	}

	return &chain.Result{
		Success: true,
		Error:   []byte{},

		Outputs: actionOutputs,

		Units: units,
		Fee:   fee,
	}, nil
}

func (DefaultTransactionExecutor) ConsumeUnits(
	tx *chain.Transaction,
	feeManager *internalfees.Manager,
	bh chain.BalanceHandler,
	r chain.Rules,
) error {
	// Ensure we don't consume too many units
	units, err := tx.Units(bh, r)
	if err != nil {
		return err
	}
	if ok, d := feeManager.Consume(units, r.GetMaxBlockUnits()); !ok {
		return fmt.Errorf("%w: %d too large", chain.ErrInvalidUnitsConsumed, d)
	}

	return nil
}
