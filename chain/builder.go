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
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

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
	ectx, err := GenerateExecutionContext(ctx, nextTime, parent, vm.Tracer(), r)
	if err != nil {
		log.Warn("block building failed: couldn't get execution context", zap.Error(err))
		return nil, err
	}
	b := NewBlock(ectx, vm, parent, nextTime)

	mempoolSize := vm.Mempool().Len(ctx)
	changesEstimate := math.Min(mempoolSize, maxViewPreallocation)
	state, err := parent.childState(ctx, changesEstimate)
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}
	ts := tstate.New(changesEstimate)

	var (
		oldestAllowed = nextTime - r.GetValidityWindow()

		surplusFee = uint64(0)
		mempool    = vm.Mempool()

		txsAttempted = 0
		results      = []*Result{}

		warpCount = 0

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
	for time.Since(start) < vm.GetTargetBuildDuration() {
		prepareStreamLock.Lock()
		txs := mempool.Stream(ctx, streamBatch)
		prepareStreamLock.Unlock()
		if len(txs) == 0 {
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
						return
					}
					alreadyFetched[sk] = &fetchData{v, true}
					storage[sk] = v
				}
				readyTxs <- &txData{tx, storage}
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
			nextUnits, err := next.MaxUnits(r)
			if err != nil {
				// Should never happen
				log.Warn(
					"skipping tx: invalid max units",
					zap.Error(err),
				)
				continue
			}
			if b.UnitsConsumed+nextUnits > r.GetMaxBlockUnits() {
				log.Debug(
					"skipping tx: too many units",
					zap.Uint64("block units", b.UnitsConsumed),
					zap.Uint64("tx max units", nextUnits),
				)
				restorable = append(restorable, next)

				// We only stop building once we are within [stopBuildingThreshold] to protect
				// against a case where there is a very large transaction in the mempool (and the
				// block is still empty).
				//
				// We are willing to give up building early (although there may be some remaining space)
				// to avoid iterating over everything in the mempool to find something that may fit.
				if r.GetMaxBlockUnits()-b.UnitsConsumed < stopBuildingThreshold {
					execErr = errBlockFull
				}
				continue
			}

			// Populate required transaction state and restrict which keys can be used
			txStart := ts.OpIndex()
			ts.SetScope(ctx, next.StateKeys(sm), nextTxData.storage)

			// PreExecute next to see if it is fit
			if err := next.PreExecute(ctx, ectx, r, ts, nextTime); err != nil {
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
			result, err := next.Execute(
				ctx,
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
