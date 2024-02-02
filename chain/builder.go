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

	"github.com/ava-labs/hypersdk/executor"
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
	parent *StatelessBlock,
	blockContext *smblock.Context,
) (*StatelessBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	// We don't need to fetch the [VerifyContext] because
	// we will always have a block to build on.

	// Select next timestamp
	nextTime := time.Now().UnixMilli()
	r := vm.Rules(nextTime)
	if nextTime < parent.Tmstmp+r.GetMinBlockGap() {
		log.Debug("block building failed", zap.Error(ErrTimestampTooEarly))
		return nil, ErrTimestampTooEarly
	}
	b := NewBlock(vm, parent, nextTime)

	// Fetch view where we will apply block state transitions
	//
	// If the parent block is not yet verified, we will attempt to
	// execute it.
	mempoolSize := vm.Mempool().Len(ctx)
	changesEstimate := math.Min(mempoolSize, maxViewPreallocation)
	parentView, err := parent.View(ctx, true)
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}

	// Compute next unit prices to use
	feeKey := FeeKey(vm.StateManager().FeeKey())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return nil, err
	}
	parentFeeManager := NewFeeManager(feeRaw)
	feeManager, err := parentFeeManager.ComputeNext(parent.Tmstmp, nextTime, r)
	if err != nil {
		return nil, err
	}
	maxUnits := r.GetMaxBlockUnits()
	targetUnits := r.GetWindowTargetUnits()

	var (
		ts            = tstate.New(changesEstimate)
		oldestAllowed = nextTime - r.GetValidityWindow()

		mempool = vm.Mempool()

		// restorable txs after block attempt finishes
		restorableLock sync.Mutex
		restorable     = []*Transaction{}

		// cache contains keys already fetched from state that can be
		// used during prefetching.
		cacheLock sync.RWMutex
		cache     = map[string]*fetchData{}

		blockLock    sync.RWMutex
		warpAdded    = uint(0)
		start        = time.Now()
		txsAttempted = 0
		results      = []*Result{}

		vdrState = vm.ValidatorState()
		sm       = vm.StateManager()

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
			b.vm.RecordClearedMempool()
			break
		}
		ctx, executeSpan := vm.Tracer().Start(ctx, "chain.BuildBlock.Execute")

		// Perform a batch repeat check
		dup, err := parent.IsRepeat(ctx, oldestAllowed, txs, set.NewBits(), false)
		if err != nil {
			restorable = append(restorable, txs...)
			break
		}

		e := executor.New(streamBatch, vm.GetTransactionExecutionCores(), vm.GetExecutorBuildRecorder())
		pending := make(map[ids.ID]*Transaction, streamBatch)
		var pendingLock sync.Mutex
		for li, ltx := range txs {
			txsAttempted++
			i := li
			tx := ltx

			// Skip any duplicates before going async
			if dup.Contains(i) {
				continue
			}

			// Ensure we can process if transaction includes a warp message
			if tx.WarpMessage != nil && blockContext == nil {
				log.Debug(
					"dropping pending warp message because no context provided",
					zap.Stringer("txID", tx.ID()),
				)
				restorableLock.Lock()
				restorable = append(restorable, tx)
				restorableLock.Unlock()
				continue
			}

			stateKeys, err := tx.StateKeys(sm)
			if err != nil {
				// Drop bad transaction and continue
				//
				// This should not happen because we check this before
				// adding a transaction to the mempool.
				continue
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

			// We track pending transactions because an error may cause us
			// not to execute restorable transactions.
			pendingLock.Lock()
			pending[tx.ID()] = tx
			pendingLock.Unlock()
			e.Run(stateKeys, func() error {
				// We use defer here instead of covering all returns because it is
				// much easier to manage.
				var restore bool
				defer func() {
					pendingLock.Lock()
					delete(pending, tx.ID())
					pendingLock.Unlock()

					if !restore {
						return
					}
					restorableLock.Lock()
					restorable = append(restorable, tx)
					restorableLock.Unlock()
				}()

				// Fetch keys from cache
				var (
					storage  = make(map[string][]byte, len(stateKeys))
					toLookup = make([]string, 0, len(stateKeys))
				)
				cacheLock.RLock()
				for k := range stateKeys {
					if v, ok := cache[k]; ok {
						if v.exists {
							storage[k] = v.v
						}
						continue
					}
					toLookup = append(toLookup, k)
				}
				cacheLock.RUnlock()

				// Fetch keys from disk
				var toCache map[string]*fetchData
				if len(toLookup) > 0 {
					toCache = make(map[string]*fetchData, len(toLookup))
					for _, k := range toLookup {
						v, err := parentView.GetValue(ctx, []byte(k))
						if errors.Is(err, database.ErrNotFound) {
							toCache[k] = &fetchData{nil, false, 0}
							continue
						} else if err != nil {
							return err
						}
						// We verify that the [NumChunks] is already less than the number
						// added on the write path, so we don't need to do so again here.
						numChunks, ok := keys.NumChunks(v)
						if !ok {
							return ErrInvalidKeyValue
						}
						toCache[k] = &fetchData{v, true, numChunks}
						storage[k] = v
					}

					// Update key cache regardless of whether exit is graceful
					defer func() {
						cacheLock.Lock()
						for k := range toCache {
							cache[k] = toCache[k]
						}
						cacheLock.Unlock()
					}()
				}

				// Execute block
				tsv := ts.NewView(stateKeys, storage)
				if err := tx.PreExecute(ctx, feeManager, sm, r, tsv, nextTime); err != nil {
					// We don't need to rollback [tsv] here because it will never
					// be committed.
					if HandlePreExecute(log, err) {
						restore = true
					}
					return nil
				}

				// Verify warp message, if it exists
				//
				// We don't drop invalid warp messages because we must collect fees for
				// the work the sender made us do (otherwise this would be a DoS).
				//
				// We wait as long as possible to verify the signature to ensure we don't
				// spend unnecessary time on an invalid tx.
				var warpErr error
				if tx.WarpMessage != nil {
					// We do not check the validity of [SourceChainID] because a VM could send
					// itself a message to trigger a chain upgrade.
					allowed, num, denom := r.GetWarpConfig(tx.WarpMessage.SourceChainID)
					if allowed {
						warpErr = tx.WarpMessage.Signature.Verify(
							ctx, &tx.WarpMessage.UnsignedMessage, r.NetworkID(),
							vdrState, blockContext.PChainHeight, num, denom,
						)
					} else {
						warpErr = ErrDisabledChainID
					}
					if warpErr != nil {
						log.Warn(
							"warp verification failed",
							zap.Stringer("txID", tx.ID()),
							zap.Error(warpErr),
						)
					}
				}

				// If execution works, keep moving forward with new state
				//
				// Note, these calculations must match block verification exactly
				// otherwise they will produce a different state root.
				blockLock.RLock()
				reads := make(map[string]uint16, len(stateKeys))
				var invalidStateKeys bool
				for k := range stateKeys {
					v := storage[k]
					numChunks, ok := keys.NumChunks(v)
					if !ok {
						invalidStateKeys = true
						break
					}
					reads[k] = numChunks
				}
				blockLock.RUnlock()
				if invalidStateKeys {
					// This should not happen because we check this before
					// adding a transaction to the mempool.
					log.Warn("invalid tx: invalid state keys")
					return nil
				}
				result, err := tx.Execute(
					ctx,
					feeManager,
					reads,
					sm,
					r,
					tsv,
					nextTime,
					tx.WarpMessage != nil && warpErr == nil,
				)
				if err != nil {
					// Returning an error here should be avoided at all costs (can be a DoS). Rather,
					// all units for the transaction should be consumed and a fee should be charged.
					log.Warn("unexpected post-execution error", zap.Error(err))
					restore = true
					return err
				}

				// Need to atomically check there aren't too many warp messages and add to block
				blockLock.Lock()
				defer blockLock.Unlock()

				// Ensure block isn't too big
				if ok, dimension := feeManager.Consume(result.Consumed, maxUnits); !ok {
					log.Debug(
						"skipping tx: too many units",
						zap.Int("dimension", int(dimension)),
						zap.Uint64("tx", result.Consumed[dimension]),
						zap.Uint64("block units", feeManager.LastConsumed(dimension)),
						zap.Uint64("max block units", maxUnits[dimension]),
					)
					restore = true

					// If we are above the target for the dimension we can't consume, we will
					// stop building. This prevents a full mempool iteration looking for the
					// "perfect fit".
					if feeManager.LastConsumed(dimension) >= targetUnits[dimension] {
						return errBlockFull
					}
				}

				// Update block with new transaction
				tsv.Commit()
				b.Txs = append(b.Txs, tx)
				results = append(results, result)
				if tx.WarpMessage != nil {
					if warpErr == nil {
						// Add a bit if the warp message was verified
						b.WarpResults.Add(warpAdded)
					}
					warpAdded++
				}
				return nil
			})
		}
		execErr := e.Wait()
		executeSpan.End()

		// Handle execution result
		if execErr != nil {
			for _, tx := range pending {
				// If we stopped executing, make sure to add those txs back
				restorable = append(restorable, tx)
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

	// Update chain metadata
	heightKey := HeightKey(sm.HeightKey())
	heightKeyStr := string(heightKey)
	timestampKey := TimestampKey(b.vm.StateManager().TimestampKey())
	timestampKeyStr := string(timestampKey)
	feeKeyStr := string(feeKey)
	tsv := ts.NewView(set.Of(heightKeyStr, timestampKeyStr, feeKeyStr), map[string][]byte{
		heightKeyStr:    binary.BigEndian.AppendUint64(nil, parent.Hght),
		timestampKeyStr: binary.BigEndian.AppendUint64(nil, uint64(parent.Tmstmp)),
		feeKeyStr:       parentFeeManager.Bytes(),
	})
	if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, fmt.Errorf("%w: unable to insert height", err)
	}
	if err := tsv.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(b.Tmstmp))); err != nil {
		return nil, fmt.Errorf("%w: unable to insert timestamp", err)
	}
	if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
		return nil, fmt.Errorf("%w: unable to insert fees", err)
	}
	tsv.Commit()

	// Fetch [parentView] root as late as possible to allow
	// for async processing to complete
	root, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	b.StateRoot = root

	// Get view from [tstate] after writing all changed keys
	view, err := ts.ExportMerkleDBView(ctx, vm.Tracer(), parentView)
	if err != nil {
		return nil, err
	}

	// Compute block hash and marshaled representation
	if err := b.initializeBuilt(ctx, view, results, feeManager); err != nil {
		log.Warn("block failed", zap.Int("txs", len(b.Txs)), zap.Any("consumed", feeManager.UnitsConsumed()))
		return nil, err
	}

	// Kickoff root generation
	go func() {
		start := time.Now()
		root, err := view.GetMerkleRoot(ctx)
		if err != nil {
			log.Error("merkle root generation failed", zap.Error(err))
			return
		}
		log.Info("merkle root generated",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
			zap.Stringer("root", root),
		)
		b.vm.RecordRootCalculated(time.Since(start))
	}()

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
