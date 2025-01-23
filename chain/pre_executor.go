// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/hypersdk/state"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

type PreExecutor struct {
	ruleFactory     RuleFactory
	validityWindow  ValidityWindow
	metadataManager MetadataManager
	balanceHandler  BalanceHandler
}

func NewPreExecutor(
	ruleFactory RuleFactory,
	validityWindow ValidityWindow,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
) *PreExecutor {
	return &PreExecutor{
		ruleFactory:     ruleFactory,
		validityWindow:  validityWindow,
		metadataManager: metadataManager,
		balanceHandler:  balanceHandler,
	}
}

func (p *PreExecutor) PreExecute(
	ctx context.Context,
	parentBlk *ExecutionBlock,
	im state.Immutable,
	tx *Transaction,
) error {
	feeRaw, err := im.GetValue(ctx, FeeKey(p.metadataManager.FeePrefix()))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedToFetchParentFee, err)
	}
	feeManager := internalfees.NewManager(feeRaw)
	now := time.Now().UnixMilli()
	r := p.ruleFactory.GetRules(now)
	nextFeeManager := feeManager.ComputeNext(now, r)

	// Find repeats
	repeatErrs, err := p.validityWindow.IsRepeat(ctx, parentBlk, now, []*Transaction{tx})
	if err != nil {
		return err
	}
	if repeatErrs.BitLen() > 0 {
		return ErrDuplicateTx
	}

	// Ensure state keys are valid
	_, err = tx.StateKeys(p.balanceHandler)
	if err != nil {
		return err
	}

	// Verify auth if not already verified by caller
	if err := tx.VerifyAuth(ctx); err != nil {
		return err
	}

	// PreExecute does not make any changes to state
	//
	// This may fail if the state we are utilizing is invalidated (if a trie
	// view from a different branch is committed underneath it). We prefer this
	// instead of putting a lock around all commits.
	//
	// Note, [PreExecute] ensures that the pending transaction does not have
	// an expiry time further ahead than [ValidityWindow]. This ensures anything
	// added to the [Mempool] is immediately executable.
	if err := tx.PreExecute(ctx, nextFeeManager, p.balanceHandler, r, im, now); err != nil {
		return err
	}
	return nil
}
