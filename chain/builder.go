// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/tstate"
)

func HandlePreExecute(
	err error,
) (bool /* continue */, bool /* restore */, bool /* remove account */) {
	if errors.Is(err, ErrInsufficientPrice) {
		return true, true, false
	}

	if errors.Is(err, ErrTimestampTooEarly) {
		return true, true, false
	}

	if errors.Is(err, ErrTimestampTooLate) {
		return true, false, false
	}

	if errors.Is(err, ErrInvalidBalance) {
		return true, false, true
	}

	if errors.Is(err, ErrAuthNotActivated) {
		return true, false, false
	}

	if errors.Is(err, ErrAuthFailed) {
		return true, false, false
	}

	if errors.Is(err, ErrActionNotActivated) {
		return true, false, false
	}

	// If unknown error, drop
	return true, false, false
}

func BuildBlock(ctx context.Context, vm VM, preferred ids.ID) (snowman.Block, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	nextTime := time.Now().Unix()
	r := vm.Rules(nextTime)
	parent, err := vm.GetStatelessBlock(ctx, preferred)
	if err != nil {
		log.Warn("block building failed: couldn't get parent", zap.Error(err))
		return nil, err
	}
	ectx, err := GenerateExecutionContext(ctx, nextTime, parent, vm.Tracer(), r)
	if err != nil {
		log.Warn("block building failed: couldn't get execution context", zap.Error(err))
		return nil, err
	}
	b := NewBlock(ectx, vm, parent, nextTime)

	state, err := parent.childState(ctx, r.GetMaxBlockTxs())
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}
	ts := tstate.New(r.GetMaxBlockTxs(), r.GetMaxBlockTxs())

	// Restorable txs after block attempt finishes
	b.Txs = []*Transaction{}
	var (
		oldestAllowed = nextTime - r.GetValidityWindow()

		surplusFee = uint64(0)
		mempool    = vm.Mempool()

		txsAttempted = 0
		results      = []*Result{}
	)
	mempoolErr := mempool.Build(
		ctx,
		func(fctx context.Context, next *Transaction) (cont bool, restore bool, removeAcct bool, err error) {
			txsAttempted++

			// Check for repeats
			//
			// TODO: check a bunch at once during pre-fetch to avoid re-walking blocks
			// for every tx
			dup, err := parent.IsRepeat(ctx, oldestAllowed, []*Transaction{next})
			if err != nil {
				return false, false, false, err
			}
			if dup {
				// tx will be restored when ancestry is rejected
				return true, false, false, nil
			}

			// Ensure we have room
			nextUnits := next.MaxUnits(r)
			if b.UnitsConsumed+nextUnits > r.GetMaxBlockUnits() {
				log.Debug(
					"skipping tx: too many units",
					zap.Uint64("block units", b.UnitsConsumed),
					zap.Uint64("tx max units", nextUnits),
				)
				return false /* make simpler */, true, false, nil // could be txs that fit that are smaller
			}

			// Populate required transaction state and restrict which keys can be used
			//
			// TODO: prefetch state of upcoming txs that we will pull (should make much
			// faster)
			txStart := ts.OpIndex()
			if err := ts.FetchAndSetScope(ctx, state, next.StateKeys()); err != nil {
				return false, true, false, err
			}

			// PreExecute next to see if it is fit
			if err := next.PreExecute(fctx, ectx, r, ts, nextTime); err != nil {
				ts.Rollback(ctx, txStart)
				cont, restore, removeAcct := HandlePreExecute(err)
				return cont, restore, removeAcct, nil
			}

			// If execution works, keep moving forward with new state
			result, err := next.Execute(fctx, r, ts, nextTime)
			if err != nil {
				// This error should only be raised by the handler, not the
				// implementation itself
				log.Warn("unexpected post-execution error", zap.Error(err))
				return false, false, false, err
			}

			// Update block with new transaction
			b.Txs = append(b.Txs, next)
			b.UnitsConsumed += result.Units
			surplusFee += (next.Base.UnitPrice - b.UnitPrice) * result.Units
			results = append(results, result)
			return len(b.Txs) < r.GetMaxBlockTxs(), false, false, nil
		},
	)
	span.SetAttributes(
		attribute.Int("attempted", txsAttempted),
		attribute.Int("added", len(b.Txs)),
	)
	if mempoolErr != nil {
		b.vm.Mempool().Add(ctx, b.Txs)
		return nil, err
	}

	// Perform basic validity checks to make sure the block is well-formatted
	if len(b.Txs) == 0 {
		return nil, ErrNoTxs
	}
	requiredSurplus := b.UnitPrice * b.BlockCost
	if surplusFee < requiredSurplus {
		// This is a very common result during block building
		b.vm.Mempool().Add(ctx, b.Txs)
		return nil, fmt.Errorf(
			"%w: required=%d found=%d",
			ErrInsufficientSurplus,
			requiredSurplus,
			surplusFee,
		)
	}
	b.SurplusFee = surplusFee

	// Get root from underlying state changes after writing all changed keys
	if err := ts.WriteChanges(ctx, state, vm.Tracer()); err != nil {
		return nil, err
	}
	root, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	b.StateRoot = root

	// Compute block hash and marshaled representation
	if err := b.init(ctx, results, false); err != nil {
		return nil, err
	}
	log.Info(
		"built block",
		zap.Uint64("hght", b.Hght),
		zap.Int("attempted", txsAttempted),
		zap.Int("added", len(b.Txs)),
		zap.Int("mempool size", b.vm.Mempool().Len(ctx)),
	)
	return b, nil
}
