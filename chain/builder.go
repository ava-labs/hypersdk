// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/math"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/tstate"
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
) (*StatelessRootBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	nextTime := time.Now().Unix()
	r := vm.Rules(nextTime)
	parent, err := vm.GetStatelessRootBlock(ctx, preferred)
	if err != nil {
		log.Warn("block building failed: couldn't get parent", zap.Error(err))
		return nil, err
	}
	var parentTxBlock *StatelessTxBlock
	if len(parent.Txs) > 0 { // if first block, will not have any tx blocks
		parentTxBlock, err = vm.GetStatelessTxBlock(ctx, parent.Txs[len(parent.Txs)-1])
		if err != nil {
			log.Warn("block building failed: couldn't get parent tx block", zap.Error(err))
			return nil, err
		}
	}
	ectx, err := GenerateRootExecutionContext(ctx, vm.ChainID(), nextTime, parent, vm.Tracer(), r)
	if err != nil {
		log.Warn("block building failed: couldn't get execution context", zap.Error(err))
		return nil, err
	}

	b := NewRootBlock(ectx, vm, parent, nextTime)

	changesEstimate := math.Min(vm.Mempool().Len(ctx), 10_000) // TODO: improve estimate
	state, err := parentTxBlock.ChildState(ctx, changesEstimate)
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}
	ts := tstate.New(changesEstimate)

	// Restorable txs after block attempt finishes
	tectx, err := GenerateTxExecutionContext(ctx, vm.ChainID(), nextTime, parentTxBlock, vm.Tracer(), r)
	if err != nil {
		return nil, err
	}
	var (
		oldestAllowed = nextTime - r.GetValidityWindow()
		mempool       = vm.Mempool()

		txBlocks = []*StatelessTxBlock{}
		txBlock  = NewTxBlock(tectx, vm, parentTxBlock, nextTime)
		results  = []*Result{}

		totalUnits   = uint64(0)
		txsAttempted = 0
		txsAdded     = 0
		warpCount    = 0

		vdrState = vm.ValidatorState()
		sm       = vm.StateManager()

		start    = time.Now()
		lockWait time.Duration
	)
	b.MinTxHght = txBlock.Hght
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
			dup, err := parentTxBlock.IsRepeat(ctx, oldestAllowed, []*Transaction{next})
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

			// Determine if we need to create a new TxBlock
			//
			// TODO: handle case where tx is larger than max size of TxBlock
			if txBlock.UnitsConsumed+nextUnits > r.GetMaxTxBlockUnits() {
				if err := ts.WriteChanges(ctx, state, vm.Tracer()); err != nil {
					return false, true, false, err
				}
				if err := state.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, txBlock.Hght)); err != nil {
					return false, true, false, err
				}
				if len(txBlocks)+1 /* account for current */ >= r.GetMaxTxBlocks() {
					txBlock.Last = true
				}
				if err := txBlock.initializeBuilt(ctx, state, results); err != nil {
					return false, true, false, err
				}
				b.Txs = append(b.Txs, txBlock.ID())
				txBlocks = append(txBlocks, txBlock)
				vm.IssueTxBlock(ctx, txBlock)
				if txBlock.Last {
					txBlock = nil
					return false, true, false, nil
				}

				state, err = txBlock.ChildState(ctx, changesEstimate)
				if err != nil {
					return false, true, false, err
				}
				ts = tstate.New(changesEstimate)
				parentTxBlock = txBlock
				tectx, err = GenerateTxExecutionContext(ctx, vm.ChainID(), nextTime, parentTxBlock, vm.Tracer(), r)
				if err != nil {
					return false, true, false, err
				}
				txBlock = NewTxBlock(tectx, vm, txBlock, nextTime)
				results = []*Result{}
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
			if err := next.PreExecute(fctx, tectx, r, ts, nextTime); err != nil {
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
				tectx,
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
			txBlock.Txs = append(txBlock.Txs, next)
			txBlock.UnitsConsumed += result.Units
			results = append(results, result)
			if next.WarpMessage != nil {
				if warpErr == nil {
					// Add a bit if the warp message was verified
					txBlock.WarpResults.Add(uint(warpCount))
				}
				warpCount++
				txBlock.ContainsWarp = true
				txBlock.PChainHeight = blockContext.PChainHeight
			}
			totalUnits += result.Units
			txsAdded++
			return true, false, false, nil
		},
	)
	span.SetAttributes(
		attribute.Int("attempted", txsAttempted),
		attribute.Int("added", len(b.Txs)),
	)
	if mempoolErr != nil {
		for _, block := range txBlocks {
			b.vm.Mempool().Add(ctx, block.Txs)
		}
		if txBlock != nil {
			b.vm.Mempool().Add(ctx, txBlock.Txs)
		}
		b.vm.Logger().Warn("build failed", zap.Error(mempoolErr))
		return nil, mempoolErr
	}

	// Create last tx block
	//
	// TODO: unify this logic with inner block tracker
	if txBlock != nil && len(txBlock.Txs) > 0 {
		txBlock.Last = true
		if err := ts.WriteChanges(ctx, state, vm.Tracer()); err != nil {
			return nil, err
		}
		if err := state.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, txBlock.Hght)); err != nil {
			return nil, err
		}
		if err := txBlock.initializeBuilt(ctx, state, results); err != nil {
			return nil, err
		}
		b.Txs = append(b.Txs, txBlock.ID())
		txBlocks = append(txBlocks, txBlock)
		vm.IssueTxBlock(ctx, txBlock)
	}

	// Perform basic validity checks to make sure the block is well-formatted
	if len(b.Txs) == 0 {
		return nil, ErrNoTxs
	}

	// Compute state root after all data has been written to trie
	root, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	b.StateRoot = root
	b.UnitsConsumed = totalUnits
	b.ContainsWarp = warpCount > 0

	// Compute block hash and marshaled representation
	if err := b.initializeBuilt(ctx, txBlocks); err != nil {
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
