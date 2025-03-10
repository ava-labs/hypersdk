// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

const (
	maxViewPreallocation = 10_000

	// TODO: make these tunable
	streamBatch             = 256
	streamPrefetchThreshold = streamBatch / 2
	stopBuildingThreshold   = 2_048 // units
)

var errBlockFull = errors.New("block full")

type Builder struct {
	tracer          trace.Tracer
	ruleFactory     RuleFactory
	log             logging.Logger
	metadataManager MetadataManager
	balanceHandler  BalanceHandler
	mempool         Mempool
	validityWindow  ValidityWindow
	metrics         *ChainMetrics
	config          Config
}

func NewBuilder(
	tracer trace.Tracer,
	ruleFactory RuleFactory,
	log logging.Logger,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
	mempool Mempool,
	validityWindow ValidityWindow,
	metrics *ChainMetrics,
	config Config,
) *Builder {
	return &Builder{
		tracer:          tracer,
		ruleFactory:     ruleFactory,
		log:             log,
		metadataManager: metadataManager,
		balanceHandler:  balanceHandler,
		mempool:         mempool,
		validityWindow:  validityWindow,
		metrics:         metrics,
		config:          config,
	}
}

func (c *Builder) BuildBlock(ctx context.Context, pChainCtx *block.Context, parentOutputBlock *OutputBlock) (*ExecutionBlock, *OutputBlock, error) {
	ctx, span := c.tracer.Start(ctx, "Chain.BuildBlock")
	defer span.End()

	parent := parentOutputBlock.ExecutionBlock
	parentView := parentOutputBlock.View

	// Select next timestamp
	nextTime := time.Now().UnixMilli()
	r := c.ruleFactory.GetRules(nextTime)
	if nextTime < parent.Tmstmp+r.GetMinBlockGap() {
		c.log.Debug("block building failed", zap.Error(ErrTimestampTooEarly))
		return nil, nil, fmt.Errorf("%w: proposed build block time (%d) < parentTimestamp (%d) + minBlockGap (%d)", ErrTimestampTooEarly, nextTime, parent.Tmstmp, r.GetMinBlockGap())
	}
	var (
		parentID          = parent.GetID()
		timestamp         = nextTime
		height            = parent.Hght + 1
		blockTransactions = []*Transaction{}
	)

	// Compute next unit prices to use
	feeKey := FeeKey(c.metadataManager.FeePrefix())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return nil, nil, err
	}
	parentFeeManager := fees.NewManager(feeRaw)
	feeManager := parentFeeManager.ComputeNext(nextTime, r)

	maxUnits := r.GetMaxBlockUnits()
	targetUnits := r.GetWindowTargetUnits()

	mempoolSize := c.mempool.Len(ctx)
	changesEstimate := min(mempoolSize, maxViewPreallocation)

	var (
		ts = tstate.New(changesEstimate)

		// restorable txs after block attempt finishes
		restorableLock sync.Mutex
		restorable     = []*Transaction{}

		// cache contains keys already fetched from state that can be
		// used during prefetching.
		cacheLock sync.RWMutex
		cache     = map[string]*fetchData{}

		blockLock    sync.RWMutex
		start        = time.Now()
		txsAttempted = 0
		results      = []*Result{}

		// prepareStreamLock ensures we don't overwrite stream prefetching spawned
		// asynchronously.
		prepareStreamLock sync.Mutex

		// stop is used to trigger that we should stop building, assuming we are no longer executing
		stop bool
	)

	// Batch fetch items from mempool to unblock incoming RPC/Gossip traffic
	c.mempool.StartStreaming(ctx)
	for time.Since(start) < c.config.TargetBuildDuration && !stop {
		prepareStreamLock.Lock()
		txs := c.mempool.Stream(ctx, streamBatch)
		prepareStreamLock.Unlock()
		if len(txs) == 0 {
			c.metrics.clearedMempool.Inc()
			break
		}
		ctx, executeSpan := c.tracer.Start(ctx, "Chain.BuildBlock.Execute")

		// Perform a batch repeat check
		// IsRepeat only returns an error if we fail to fetch the full validity window of blocks.
		// This should only happen after startup, so we add the transactions back to the mempool.
		dup, err := c.validityWindow.IsRepeat(ctx, parent, nextTime, txs)
		if err != nil {
			restorable = append(restorable, txs...)
			break
		}

		e := executor.New(streamBatch, c.config.TransactionExecutionCores, MaxKeyDependencies, c.metrics.executorBuildRecorder)
		pending := make(map[ids.ID]*Transaction, streamBatch)
		var pendingLock sync.Mutex
		totalTxsSize := 0
		for li, ltx := range txs {
			txsAttempted++
			totalTxsSize += ltx.Size()
			if totalTxsSize > c.config.TargetTxsSize {
				c.log.Debug("Transactions in block exceeded allotted limit ", zap.Int("size", ltx.Size()))
				restorable = append(restorable, txs[li:]...)
				break
			}

			i := li
			tx := ltx

			// Skip any duplicates before going async
			if dup.Contains(i) {
				continue
			}

			stateKeys, err := tx.StateKeys(c.balanceHandler)
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
					c.mempool.PrepareStream(ctx, streamBatch)
					prepareStreamLock.Unlock()
				}()
			}

			// We track pending transactions because an error may cause us
			// not to execute restorable transactions.
			pendingLock.Lock()
			pending[tx.GetID()] = tx
			pendingLock.Unlock()
			e.Run(stateKeys, func() error {
				// We use defer here instead of covering all returns because it is
				// much easier to manage.
				var restore bool
				defer func() {
					pendingLock.Lock()
					delete(pending, tx.GetID())
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
				tsv := ts.NewView(
					stateKeys,
					state.ImmutableStorage(storage),
					len(stateKeys),
				)
				if err := tx.PreExecute(ctx, feeManager, c.balanceHandler, r, tsv, nextTime); err != nil {
					// Ignore the error and drop the transaction.
					// We don't need to rollback [tsv] here because it will never
					// be committed.
					return nil
				}
				result, err := tx.Execute(
					ctx,
					feeManager,
					c.balanceHandler,
					r,
					tsv,
					nextTime,
				)
				if err != nil {
					// Returning an error here should be avoided at all costs (can be a DoS). Rather,
					// all units for the transaction should be consumed and a fee should be charged.
					c.log.Warn("unexpected post-execution error", zap.Error(err))
					restore = true
					return err
				}

				blockLock.Lock()
				defer blockLock.Unlock()

				// Ensure block isn't too big
				if ok, dimension := feeManager.Consume(result.Units, maxUnits); !ok {
					c.log.Debug(
						"skipping tx: too many units",
						zap.Int("dimension", int(dimension)),
						zap.Uint64("tx", result.Units[dimension]),
						zap.Uint64("block units", feeManager.LastConsumed(dimension)),
						zap.Uint64("max block units", maxUnits[dimension]),
					)
					restore = true

					// If we are above the target for the dimension we can't consume, we will
					// stop building. This prevents a full mempool iteration looking for the
					// "perfect fit".
					if feeManager.LastConsumed(dimension) >= targetUnits[dimension] {
						stop = true
						return errBlockFull
					}
					// Skip this transaction and continue packing if it exceeds the dimension's
					// limit.
					return nil
				}

				// Update block with new transaction
				tsv.Commit()
				blockTransactions = append(blockTransactions, tx)
				results = append(results, result)
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
					restored := c.mempool.FinishStreaming(ctx, append(blockTransactions, restorable...))
					c.log.Debug("transactions restored to mempool", zap.Int("count", restored))
				}()
				c.log.Warn("build failed", zap.Error(execErr))
				return nil, nil, execErr
			}
			break
		}
	}

	// Wait for stream preparation to finish to make
	// sure all transactions are returned to the mempool.
	go func() {
		prepareStreamLock.Lock()
		restored := c.mempool.FinishStreaming(ctx, restorable)
		c.log.Debug("transactions restored to mempool", zap.Int("count", restored))
	}()

	// Update tracking metrics
	span.SetAttributes(
		attribute.Int("attempted", txsAttempted),
		attribute.Int("added", len(blockTransactions)),
	)
	if time.Since(start) > c.config.TargetBuildDuration {
		c.metrics.buildCapped.Inc()
	}

	// Perform basic validity checks to make sure the block is well-formatted
	if len(blockTransactions) == 0 {
		if nextTime < parent.Tmstmp+r.GetMinEmptyBlockGap() {
			return nil, nil, fmt.Errorf("%w: allowed in %d ms", ErrNoTxs, parent.Tmstmp+r.GetMinEmptyBlockGap()-nextTime)
		}
		c.metrics.emptyBlockBuilt.Inc()
	}

	// Update chain metadata
	heightKey := HeightKey(c.metadataManager.HeightPrefix())
	heightKeyStr := string(heightKey)
	timestampKey := TimestampKey(c.metadataManager.TimestampPrefix())
	timestampKeyStr := string(timestampKey)
	feeKeyStr := string(feeKey)

	keys := make(state.Keys)
	keys.Add(heightKeyStr, state.Write)
	keys.Add(timestampKeyStr, state.Write)
	keys.Add(feeKeyStr, state.Write)
	tsv := ts.NewView(
		keys,
		state.ImmutableStorage(map[string][]byte{
			heightKeyStr:    binary.BigEndian.AppendUint64(nil, parent.Hght),
			timestampKeyStr: binary.BigEndian.AppendUint64(nil, uint64(parent.Tmstmp)),
			feeKeyStr:       parentFeeManager.Bytes(),
		}),
		len(keys),
	)
	if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, height)); err != nil {
		return nil, nil, fmt.Errorf("%w: unable to insert height", err)
	}
	if err := tsv.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(timestamp))); err != nil {
		return nil, nil, fmt.Errorf("%w: unable to insert timestamp", err)
	}
	if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
		return nil, nil, fmt.Errorf("%w: unable to insert fees", err)
	}
	tsv.Commit()

	// Fetch [parentView] root as late as possible to allow
	// for async processing to complete
	parentStateRoot, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Calculate new view from parent and state diff
	view, err := createView(ctx, c.tracer, parentView, ts.ChangedKeys())
	if err != nil {
		return nil, nil, err
	}

	// Initialize finalized metadata fields
	blk, err := NewStatelessBlock(
		parentID,
		timestamp,
		height,
		blockTransactions,
		parentStateRoot,
		pChainCtx,
	)
	if err != nil {
		return nil, nil, err
	}

	// Kickoff root generation
	go func() {
		start := time.Now()
		root, err := view.GetMerkleRoot(ctx)
		if err != nil {
			c.log.Error("merkle root generation failed", zap.Error(err))
			return
		}
		c.log.Info("merkle root generated",
			zap.Uint64("height", blk.Hght),
			zap.Stringer("blkID", blk.GetID()),
			zap.Stringer("root", root),
		)
		c.metrics.rootCalculatedCount.Inc()
		c.metrics.rootCalculatedSum.Add(float64(time.Since(start)))
	}()

	c.metrics.txsBuilt.Add(float64(len(blockTransactions)))
	c.log.Info(
		"built block",
		zap.Uint64("hght", height),
		zap.Int("attempted", txsAttempted),
		zap.Int("added", len(blockTransactions)),
		zap.Int("state changes", ts.PendingChanges()),
		zap.Int("state operations", ts.OpIndex()),
		zap.Int64("parent (t)", parent.Tmstmp),
		zap.Int64("block (t)", timestamp),
	)
	execBlock := NewExecutionBlock(blk)
	return execBlock, &OutputBlock{
		ExecutionBlock: execBlock,
		View:           view,
		ExecutionResults: &ExecutionResults{
			Results:       results,
			UnitPrices:    feeManager.UnitPrices(),
			UnitsConsumed: feeManager.UnitsConsumed(),
		},
	}, nil
}
