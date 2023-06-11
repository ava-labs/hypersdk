// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/math"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/tstate"
)

const txBatchSize = 4_096

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

// TODO: add a min build time where we just listen for txs
func BuildBlock(
	ctx context.Context,
	vm VM,
	preferred ids.ID,
	blockContext *smblock.Context,
) (*StatelessRootBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	// TODO: migrate to milli
	// nextTime := time.Now().UnixMilli()
	nextTime := time.Now().Unix()
	r := vm.Rules(nextTime)
	parent, err := vm.GetStatelessRootBlock(ctx, preferred)
	if err != nil {
		log.Warn("block building failed: couldn't get parent", zap.Error(err))
		return nil, err
	}
	ectx, err := GenerateRootExecutionContext(ctx, vm.ChainID(), nextTime, parent, vm.Tracer(), r)
	if err != nil {
		log.Warn("block building failed: couldn't get execution context", zap.Error(err))
		return nil, err
	}

	b := NewRootBlock(ectx, vm, parent, nextTime)

	changesEstimate := math.Min(vm.Mempool().Len(ctx), 10_000) // TODO: improve estimate
	parentTxBlock, err := parent.LastTxBlock()
	if err != nil {
		log.Warn("block building failed: couldn't get parent tx block", zap.Error(err))
		return nil, err
	}
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

		start          = time.Now()
		alreadyFetched = map[string]*fetchData{}
	)
	b.MinTxHght = txBlock.Hght

	mempool.StartBuild(ctx)
	for txBlock != nil && (time.Since(start) < vm.GetMinBuildTime() || mempool.Len(ctx) > 0) && time.Since(start) < vm.GetMaxBuildTime() {
		var execErr error
		txs := mempool.LeaseItems(ctx, txBatchSize)
		if len(txs) == 0 {
			mempool.ClearLease(ctx, nil, nil)
			time.Sleep(25 * time.Millisecond)
			continue
		}

		// prefetch all lease items
		//
		// TODO: move this to a function
		readyTxs := make(chan *txData, len(txs))
		stopIndex := -1
		var timeExceeded bool
		go func() {
			// Store required keys for each set
			for i, tx := range txs {
				if txBlock == nil || timeExceeded {
					// This happens when at last tx block
					stopIndex = i
					close(readyTxs)
					return
				}
				storage := map[string][]byte{}
				for _, k := range tx.StateKeys(sm) {
					sk := string(k)
					if v, ok := alreadyFetched[sk]; ok {
						if v.exists {
							storage[sk] = v.v
						}
						continue
					}
					v, err := state.GetValue(ctx, k)
					if errors.Is(err, database.ErrNotFound) {
						alreadyFetched[sk] = &fetchData{nil, false}
						continue
					} else if err != nil {
						// This can happen if the underlying view changes (if we are
						// verifying a block that can never be accepted).
						execErr = err
						stopIndex = i
						close(readyTxs)
						return
					}
					alreadyFetched[sk] = &fetchData{v, true}
					storage[sk] = v
				}
				readyTxs <- &txData{tx, storage}
			}

			// Let caller know all sets have been readied
			close(readyTxs)
		}()

		restorable := make([]*Transaction, 0, txBatchSize)
		exempt := make([]*Transaction, 0, 10)
		for nextTx := range readyTxs {
			next := nextTx.tx

			// Avoid bulding if there is an error
			if txBlock == nil || execErr != nil {
				restorable = append(restorable, next)
				continue
			}

			// Avoid building for too long
			if time.Since(start) > vm.GetMaxBuildTime() {
				timeExceeded = true
				restorable = append(restorable, next)
				continue
			}
			txsAttempted++

			// Ensure we can process if transaction includes a warp message
			if next.WarpMessage != nil && blockContext == nil {
				log.Info(
					"dropping pending warp message because no context provided",
					zap.Stringer("txID", next.ID()),
				)
				if next.Base.Timestamp > oldestAllowed {
					restorable = append(restorable, next)
					continue
				}
			}

			// Skip warp message if at max
			if next.WarpMessage != nil && warpCount == MaxWarpMessages {
				log.Info(
					"dropping pending warp message because already have MaxWarpMessages",
					zap.Stringer("txID", next.ID()),
				)
				exempt = append(exempt, next)
				continue
			}

			// Check for repeats
			//
			// TODO: check a bunch at once during pre-fetch to avoid re-walking blocks
			// for every tx
			dup, err := parentTxBlock.IsRepeat(ctx, oldestAllowed, []*Transaction{next})
			if err != nil {
				restorable = append(restorable, next)
				execErr = err
				continue // need to finish processing txs
			}
			if dup {
				continue
			}

			// Ensure we have room
			nextUnits, err := next.MaxUnits(r)
			if err != nil {
				// Should never happen
				log.Debug(
					"skipping invalid tx",
					zap.Error(err),
				)
				continue
			}

			// Determine if we need to create a new TxBlock
			//
			// TODO: handle case where tx is larger than max size of TxBlock
			if txBlock.UnitsConsumed+nextUnits > r.GetMaxTxBlockUnits() {
				if len(txBlocks)+1 /* account for current */ >= r.GetMaxTxBlocks() {
					if err := ts.WriteChanges(ctx, state, vm.Tracer()); err != nil {
						restorable = append(restorable, next)
						execErr = err
						continue
					}
					if err := state.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, txBlock.Hght)); err != nil {
						restorable = append(restorable, next)
						execErr = err
						continue
					}
					txBlock.Last = true
				}
				txBlock.Issued = time.Now().UnixMilli()
				if err := txBlock.initializeBuilt(ctx, state, results); err != nil {
					restorable = append(restorable, next)
					execErr = err
					continue
				}
				b.Txs = append(b.Txs, txBlock.ID())
				txBlocks = append(txBlocks, txBlock)
				vm.IssueTxBlock(ctx, txBlock)
				if txBlock.Last {
					txBlock = nil
					restorable = append(restorable, next)
					continue
				}
				parentTxBlock = txBlock
				tectx, err = GenerateTxExecutionContext(ctx, vm.ChainID(), nextTime, parentTxBlock, vm.Tracer(), r)
				if err != nil {
					restorable = append(restorable, next)
					execErr = err
					continue
				}
				txBlock = NewTxBlock(tectx, vm, parentTxBlock, nextTime)
				results = []*Result{}
			}

			// Populate required transaction state and restrict which keys can be used
			txStart := ts.OpIndex()
			ts.SetScope(ctx, next.StateKeys(sm), nextTx.storage)

			// PreExecute next to see if it is fit
			if err := next.PreExecute(ctx, tectx, r, ts, nextTime); err != nil {
				ts.Rollback(ctx, txStart)
				cont, restore, _ := HandlePreExecute(err)
				if !cont {
					restorable = append(restorable, next)
					execErr = err
					continue
				}
				if restore {
					restorable = append(restorable, next)
				}
				continue
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
				ctx,
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
				restorable = append(restorable, next)
				execErr = err
				continue
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
		}
		if stopIndex >= 0 {
			// If we stopped prefetching, make sure to add those txs back
			restorable = append(restorable, txs[stopIndex:]...)
		}
		mempool.ClearLease(ctx, restorable, exempt)
		if execErr != nil {
			for _, block := range txBlocks {
				b.vm.Mempool().Add(ctx, block.Txs)
			}
			if txBlock != nil {
				b.vm.Mempool().Add(ctx, txBlock.Txs)
			}
			mempool.FinishBuild(ctx)
			b.vm.Logger().Warn("build failed", zap.Error(execErr))
			return nil, execErr
		}
	}
	mempool.FinishBuild(ctx)

	// Record if went to the limit
	if time.Since(start) >= vm.GetMaxBuildTime() {
		vm.RecordEarlyBuildStop()
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
		txBlock.Issued = time.Now().UnixMilli()
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
	b.Issued = time.Now().UnixMilli()

	// Compute block hash and marshaled representation
	if err := b.initializeBuilt(ctx, txBlocks); err != nil {
		return nil, err
	}
	mempoolSize := b.vm.Mempool().Len(ctx)
	vm.RecordMempoolSizeAfterBuild(mempoolSize)
	log.Info(
		"built block",
		zap.Uint64("hght", b.Hght),
		zap.Int("attempted", txsAttempted),
		zap.Int("added", len(b.Txs)),
		zap.Int("mempool size", mempoolSize),
		zap.Bool("context", blockContext != nil),
		zap.Int("state changes", ts.PendingChanges()),
		zap.Int("state operations", ts.OpIndex()),
	)
	return b, nil
}
