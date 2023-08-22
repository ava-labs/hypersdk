// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/tstate"
)

const (
	maxViewPreallocation = 10_000

	// TODO: make these tunable
	streamBatch             = 256
	streamPrefetchThreshold = streamBatch / 2
	stopBuildingThreshold   = 2_048 // units
)

var errBlockFull = errors.New("block full")

func HandlePreExecute(log logging.Logger, err error) bool {
	switch {
	case errors.Is(err, ErrInsufficientPrice):
		return false
	case errors.Is(err, ErrTimestampTooEarly):
		return true
	case errors.Is(err, ErrTimestampTooLate):
		return false
	case errors.Is(err, ErrInvalidBalance):
		return false
	case errors.Is(err, ErrAuthNotActivated):
		return false
	case errors.Is(err, ErrAuthFailed):
		return false
	case errors.Is(err, ErrActionNotActivated):
		return false
	default:
		// If unknown error, drop
		log.Warn("unknown PreExecute error", zap.Error(err))
		return false
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

	// Setup new block
	parent, err := vm.GetStatelessBlock(ctx, preferred)
	if err != nil {
		log.Warn("block building failed: couldn't get parent", zap.Error(err))
		return nil, err
	}
	nextTime := time.Now().UnixMilli()
	r := vm.Rules(nextTime)
	if nextTime < parent.Tmstmp+r.GetMinBlockGap() {
		log.Warn("block building failed", zap.Error(ErrTimestampTooEarly))
		return nil, ErrTimestampTooEarly
	}
	b := NewBlock(vm, parent, nextTime)

	// Fetch state to build on
	mempoolSize := vm.Mempool().Len(ctx)
	changesEstimate := math.Min(mempoolSize, maxViewPreallocation)
	state, err := parent.childState(ctx, changesEstimate)
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}

	// Compute next unit prices to use
	feeRaw, err := state.GetValue(ctx, vm.StateManager().FeeKey())
	if err != nil {
		return nil, err
	}
	feeManager := NewFeeManager(feeRaw)
	nextFeeManager, err := feeManager.ComputeNext(parent.Tmstmp, nextTime, r)
	if err != nil {
		return nil, err
	}
	maxUnits := r.GetMaxBlockUnits()
	targetUnits := r.GetWindowTargetUnits()

	ts := tstate.New(changesEstimate)

	var (
		oldestAllowed = nextTime - r.GetValidityWindow()

		mempool = vm.Mempool()

		txsAttempted = 0
		results      = []*Result{}
		warpCount    = 0

		vdrState = vm.ValidatorState()
		sm       = vm.StateManager()

		start = time.Now()

		// restorable txs after block attempt finishes
		restorable = []*Transaction{}

		// alreadyFetched contains keys already fetched from state that can be
		// used during prefetching.
		alreadyFetched = map[string]*fetchData{}

		// prepareStreamLock ensures we don't overwrite stream prefetching spawned
		// asynchronously.
		prepareStreamLock sync.Mutex
	)

	// Batch fetch items from mempool to unblock incoming RPC/Gossip traffic
	mempool.StartStreaming(ctx)
	b.Txs = []*Transaction{}
	usedKeys := set.NewSet[string](0) // prefetch map for transactions in block
	for time.Since(start) < vm.GetTargetBuildDuration() {
		prepareStreamLock.Lock()
		txs := mempool.Stream(ctx, streamBatch)
		prepareStreamLock.Unlock()
		if len(txs) == 0 {
			b.vm.RecordClearedMempool()
			break
		}

		// Prefetch all transactions
		//
		// TODO: unify logic with https://github.com/ava-labs/hypersdk/blob/4e10b911c3cd88e0ccd8d9de5210515b1d3a3ac4/chain/processor.go#L44-L79
		var (
			readyTxs  = make(chan *txData, len(txs))
			stopIndex = -1
			execErr   error
		)
		go func() {
			ctx, prefetchSpan := vm.Tracer().Start(ctx, "chain.BuildBlock.Prefetch")
			defer prefetchSpan.End()
			defer close(readyTxs)

			for i, tx := range txs {
				if execErr != nil {
					stopIndex = i
					return
				}

				// Once we get part way through a prefetching job, we start
				// to prepare for the next stream.
				if i == streamPrefetchThreshold {
					prepareStreamLock.Lock()
					go func() {
						mempool.PrepareStream(ctx, streamBatch)
						prepareStreamLock.Unlock()
					}()
				}

				// Prefetch all values from state
				storage := map[string][]byte{}
				stateKeys, err := tx.StateKeys(sm)
				if err != nil {
					// Drop bad transaction and continue
					//
					// This should not happen because we check this before
					// adding a transaction to the mempool.
					continue
				}
				for k := range stateKeys {
					if v, ok := alreadyFetched[k]; ok {
						if v.exists {
							storage[k] = v.v
						}
						continue
					}
					v, err := state.GetValue(ctx, []byte(k))
					if errors.Is(err, database.ErrNotFound) {
						alreadyFetched[k] = &fetchData{nil, false, 0}
						continue
					} else if err != nil {
						// This can happen if the underlying view changes (if we are
						// verifying a block that can never be accepted).
						execErr = err
						stopIndex = i
						return
					}
					numChunks, ok := keys.NumChunks(v)
					if !ok {
						// Drop bad transaction and continue
						//
						// This should not happen because we check this before
						// adding a transaction to the mempool.
						continue
					}
					alreadyFetched[k] = &fetchData{v, true, numChunks}
					storage[k] = v
				}
				readyTxs <- &txData{tx, storage, nil, nil}
			}
		}()

		// Perform a batch repeat check while we are waiting for state prefetching
		dup, err := parent.IsRepeat(ctx, oldestAllowed, txs, set.NewBits(), false)
		if err != nil {
			execErr = err
		}

		// Execute transactions as they become ready
		ctx, executeSpan := vm.Tracer().Start(ctx, "chain.BuildBlock.Execute")
		txIndex := 0
		for nextTxData := range readyTxs {
			txsAttempted++
			next := nextTxData.tx
			if execErr != nil {
				restorable = append(restorable, next)
				continue
			}

			// Skip if tx is a duplicate
			if dup.Contains(txIndex) {
				continue
			}
			txIndex++

			// Ensure we can process if transaction includes a warp message
			if next.WarpMessage != nil && blockContext == nil {
				log.Info(
					"dropping pending warp message because no context provided",
					zap.Stringer("txID", next.ID()),
				)
				restorable = append(restorable, next)
				continue
			}

			// Skip warp message if at max
			if next.WarpMessage != nil && warpCount == MaxWarpMessages {
				log.Info(
					"dropping pending warp message because already have MaxWarpMessages",
					zap.Stringer("txID", next.ID()),
				)
				restorable = append(restorable, next)
				continue
			}

			// Ensure we have room
			nextUnits, err := next.MaxUnits(sm, r)
			if err != nil {
				// Should never happen
				log.Warn(
					"skipping tx: invalid max units",
					zap.Error(err),
				)
				continue
			}
			if ok, dimension := nextFeeManager.CanConsume(nextUnits, maxUnits); !ok {
				log.Debug(
					"skipping tx: too many units",
					zap.Int("dimension", int(dimension)),
					zap.Uint64("tx", nextUnits[dimension]),
					zap.Uint64("block units", nextFeeManager.LastConsumed(dimension)),
					zap.Uint64("max block units", maxUnits[dimension]),
				)
				restorable = append(restorable, next)

				// If we are above the target for the dimension we can't consume, we will
				// stop building. This prevents a full mempool iteration looking for the
				// "perfect fit".
				if nextFeeManager.LastConsumed(dimension) >= targetUnits[dimension] {
					execErr = errBlockFull
				}
				continue
			}

			// Populate required transaction state and restrict which keys can be used
			txStart := ts.OpIndex()
			stateKeys, err := next.StateKeys(sm)
			if err != nil {
				// This should not happen because we check this before
				// adding a transaction to the mempool.
				log.Warn(
					"skipping tx: invalid stateKeys",
					zap.Error(err),
				)
				continue
			}
			ts.SetScope(ctx, stateKeys, nextTxData.storage)

			// PreExecute next to see if it is fit
			preExecuteStart := ts.OpIndex()
			authCUs, err := next.PreExecute(ctx, nextFeeManager, sm, r, ts, nextTime)
			if err != nil {
				ts.Rollback(ctx, txStart)
				if HandlePreExecute(log, err) {
					restorable = append(restorable, next)
				}
				continue
			}
			if preExecuteStart != ts.OpIndex() {
				// This should not happen because we check this before
				// adding a transaction to the mempool.
				log.Warn(
					"skipping tx: preexecute mutates",
					zap.Error(err),
				)
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
				// We do not check the validity of [SourceChainID] because a VM could send
				// itself a message to trigger a chain upgrade.
				allowed, num, denom := r.GetWarpConfig(next.WarpMessage.SourceChainID)
				if allowed {
					warpErr = next.WarpMessage.Signature.Verify(
						ctx, &next.WarpMessage.UnsignedMessage, r.NetworkID(),
						vdrState, blockContext.PChainHeight, num, denom,
					)
				} else {
					warpErr = ErrDisabledChainID
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
			//
			// Note, these calculations must match block verification exactly
			// otherwise they will produce a different state root.
			coldReads := map[string]uint16{}
			warmReads := map[string]uint16{}
			var invalidStateKeys bool
			for k := range stateKeys {
				v := nextTxData.storage[k]
				numChunks, ok := keys.NumChunks(v)
				if !ok {
					invalidStateKeys = true
					break
				}
				if usedKeys.Contains(k) {
					warmReads[k] = numChunks
					continue
				}
				coldReads[k] = numChunks
			}
			if invalidStateKeys {
				// This should not happen because we check this before
				// adding a transaction to the mempool.
				log.Warn("invalid tx: invalid state keys")
				continue
			}
			result, err := next.Execute(
				ctx,
				nextFeeManager,
				authCUs,
				coldReads,
				warmReads,
				sm,
				r,
				ts,
				nextTime,
				next.WarpMessage != nil && warpErr == nil,
			)
			if err != nil {
				// Returning an error here should be avoided at all costs (can be a DoS). Rather,
				// all units for the transaction should be consumed and a fee should be charged.
				log.Warn("unexpected post-execution error", zap.Error(err))
				restorable = append(restorable, next)
				execErr = err
				continue
			}

			// Update block with new transaction
			b.Txs = append(b.Txs, next)
			usedKeys.Add(stateKeys.List()...)
			if err := nextFeeManager.Consume(nextUnits); err != nil {
				execErr = err
				continue
			}
			results = append(results, result)
			if next.WarpMessage != nil {
				if warpErr == nil {
					// Add a bit if the warp message was verified
					b.WarpResults.Add(uint(warpCount))
				}
				warpCount++
			}
		}
		executeSpan.End()

		// Handle execution result
		if execErr != nil {
			if stopIndex >= 0 {
				// If we stopped prefetching, make sure to add those txs back
				restorable = append(restorable, txs[stopIndex:]...)
			}
			if !errors.Is(execErr, errBlockFull) {
				// Wait for stream preparation to finish to make
				// sure all transactions are returned to the mempool.
				go func() {
					prepareStreamLock.Lock() // we never need to unlock this as it will not be used after this
					restored := mempool.FinishStreaming(ctx, append(b.Txs, restorable...))
					b.vm.Logger().Debug("transactions restored to mempool", zap.Int("count", restored))
				}()
				b.vm.Logger().Warn("build failed", zap.Error(execErr))
				return nil, execErr
			}
			break
		}
	}

	// Wait for stream preparation to finish to make
	// sure all transactions are returned to the mempool.
	go func() {
		prepareStreamLock.Lock()
		restored := mempool.FinishStreaming(ctx, restorable)
		b.vm.Logger().Debug("transactions restored to mempool", zap.Int("count", restored))
	}()

	// Update tracking metrics
	span.SetAttributes(
		attribute.Int("attempted", txsAttempted),
		attribute.Int("added", len(b.Txs)),
	)
	if time.Since(start) > b.vm.GetTargetBuildDuration() {
		b.vm.RecordBuildCapped()
	}

	// Perform basic validity checks to make sure the block is well-formatted
	if len(b.Txs) == 0 {
		if nextTime < parent.Tmstmp+r.GetMinEmptyBlockGap() {
			return nil, fmt.Errorf("%w: allowed in %d ms", ErrNoTxs, parent.Tmstmp+r.GetMinEmptyBlockGap()-nextTime)
		}
		vm.RecordEmptyBlockBuilt()
	}

	// Get root from underlying state changes after writing all changed keys
	if err := ts.WriteChanges(ctx, state, vm.Tracer()); err != nil {
		return nil, err
	}

	// Store height in state to prevent duplicate roots
	if err := state.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, err
	}

	// Store fee parameters
	if err := state.Insert(ctx, sm.FeeKey(), nextFeeManager.Bytes()); err != nil {
		return nil, err
	}

	// Compute state root after all data has been written to trie
	root, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	b.StateRoot = root

	// Compute block hash and marshaled representation
	if err := b.initializeBuilt(ctx, state, results, nextFeeManager); err != nil {
		return nil, err
	}
	log.Info(
		"built block",
		zap.Bool("context", blockContext != nil),
		zap.Uint64("hght", b.Hght),
		zap.Int("attempted", txsAttempted),
		zap.Int("added", len(b.Txs)),
		zap.Int("state changes", ts.PendingChanges()),
		zap.Int("state operations", ts.OpIndex()),
		zap.Int64("parent (t)", parent.Tmstmp),
		zap.Int64("block (t)", b.Tmstmp),
	)
	return b, nil
}
