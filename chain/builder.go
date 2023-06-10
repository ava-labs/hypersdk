// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/math"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/tstate"
)

func HandlePreExecute(
	err error,
) (bool /* continue */, bool /* restore */, bool /* remove account */) {
	switch {
	case errors.Is(err, ErrInsufficientPrice):
		return true, true, false
	case errors.Is(err, ErrTimestampTooEarly):
		return true, true, false
	case errors.Is(err, ErrTimestampTooLate):
		return true, false, false
	case errors.Is(err, ErrInvalidBalance):
		return true, false, true
	case errors.Is(err, ErrAuthNotActivated):
		return true, false, false
	case errors.Is(err, ErrAuthFailed):
		return true, false, false
	case errors.Is(err, ErrActionNotActivated):
		return true, false, false
	default:
		// If unknown error, drop
		return true, false, false
	}
}

func BuildBlock(
	ctx context.Context,
	vm VM,
	preferred ids.ID,
	blockContext *smblock.Context,
) (*StatelessBlock, error) {
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
	ectx, err := GenerateExecutionContext(ctx, vm.ChainID(), nextTime, parent, vm.Tracer(), r)
	if err != nil {
		log.Warn("block building failed: couldn't get execution context", zap.Error(err))
		return nil, err
	}
	b := NewBlock(ectx, vm, parent, nextTime)

	changesEstimate := math.Min(vm.Mempool().Len(ctx), r.GetMaxBlockTxs())
	state, err := parent.childState(ctx, changesEstimate)
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}
	ts := tstate.New(changesEstimate)

	// Restorable txs after block attempt finishes
	b.Txs = []*Transaction{}
	var (
		oldestAllowed = nextTime - r.GetValidityWindow()

		surplusFee = uint64(0)
		mempool    = vm.Mempool()

		txsAttempted = 0
		results      = []*Result{}

		warpCount = 0

		vdrState = vm.ValidatorState()
		sm       = vm.StateManager()

		start    = time.Now()
		lockWait time.Duration
	)
	mempoolErr := mempool.Build(
		ctx,
		func(fctx context.Context, next *Transaction) (cont bool, restore bool, removeAcct bool, err error) {
			if txsAttempted == 0 {
				lockWait = time.Since(start)
			}
			txsAttempted++

			// Ensure we can process if transaction includes a warp message
			if next.WarpMessage != nil && blockContext == nil {
				log.Info(
					"dropping pending warp message because no context provided",
					zap.Stringer("txID", next.ID()),
				)
				return true, next.Base.Timestamp > oldestAllowed, false, nil
			}

			// Skip warp message if at max
			if next.WarpMessage != nil && warpCount == MaxWarpMessages {
				log.Info(
					"dropping pending warp message because already have MaxWarpMessages",
					zap.Stringer("txID", next.ID()),
				)
				return true, true, false, nil
			}

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
			nextUnits, err := next.MaxUnits(r)
			if err != nil {
				// Should never happen
				log.Debug(
					"skipping invalid tx",
					zap.Error(err),
				)
				return true, false, false, nil
			}
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
			if err := ts.FetchAndSetScope(ctx, next.StateKeys(sm), state); err != nil {
				return false, true, false, err
			}

			// PreExecute next to see if it is fit
			if err := next.PreExecute(fctx, ectx, r, ts, nextTime); err != nil {
				ts.Rollback(ctx, txStart)
				cont, restore, removeAcct := HandlePreExecute(err)
				return cont, restore, removeAcct, nil
			}

			// Verify warp message, if it exists
			//
			// We don't drop invalid warp messages because we must collect fees for
			// the work the sender made us do (otherwise this would be a DoS).
			//
			// We wait as long as possible to verify the signature to ensure we don't
			// spend unnecessary time on an invalid tx.
			var warpErr error
			if next.WarpMessage != nil {
				num, denom, err := preVerifyWarpMessage(next.WarpMessage, vm.ChainID(), r)
				if err == nil {
					warpErr = next.WarpMessage.Signature.Verify(
						ctx, &next.WarpMessage.UnsignedMessage,
						vdrState, blockContext.PChainHeight, num, denom,
					)
				} else {
					warpErr = err
				}
				if warpErr != nil {
					log.Warn(
						"warp verification failed",
						zap.Stringer("txID", next.ID()),
						zap.Error(warpErr),
					)
				}
			}

			// If execution works, keep moving forward with new state
			result, err := next.Execute(
				fctx,
				ectx,
				r,
				sm,
				ts,
				nextTime,
				next.WarpMessage != nil && warpErr == nil,
			)
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
			if next.WarpMessage != nil {
				if warpErr == nil {
					// Add a bit if the warp message was verified
					b.WarpResults.Add(uint(warpCount))
				}
				warpCount++
			}
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

	// Store height in state to prevent duplicate roots
	if err := state.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, err
	}

	// Compute state root after all data has been written to trie
	root, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	b.StateRoot = root

	// Compute block hash and marshaled representation
	if err := b.initializeBuilt(ctx, state, results); err != nil {
		return nil, err
	}
	log.Info(
		"built block",
		zap.Uint64("hght", b.Hght),
		zap.Int("attempted", txsAttempted),
		zap.Int("added", len(b.Txs)),
		zap.Int("mempool size", b.vm.Mempool().Len(ctx)),
		zap.Duration("mempool lock wait", lockWait),
		zap.Bool("context", blockContext != nil),
		zap.Int("state changes", ts.PendingChanges()),
		zap.Int("state operations", ts.OpIndex()),
	)
	return b, nil
}
