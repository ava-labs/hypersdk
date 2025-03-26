// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/fetcher"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/x/dsmr"
	"go.uber.org/zap"
)

type Assembler struct {
	tracer                  trace.Tracer
	ruleFactory             RuleFactory
	log                     logging.Logger
	metadataManager         MetadataManager
	balanceHandler          BalanceHandler
	mempool                 Mempool
	validityWindow          ValidityWindow
	authVM                  AuthVM
	authVerificationWorkers workers.Workers
	metrics                 *ChainMetrics
	config                  Config

	txParser        Parser
	batchedTxParser BatchedTransactionSerializer
}

func (a *Assembler) Assemble(
	ctx context.Context,
	parentBlock *OutputBlock,
	height uint64,
	timestamp int64,
	pChainBlockContext *block.Context,
	chunks []*dsmr.Chunk,
) (*OutputBlock, error) {
	txs, err := a.collectTxs(ctx, parentBlock, timestamp, chunks)
	if err != nil {
		return nil, err
	}

	rules := a.ruleFactory.GetRules(timestamp)

	// Create the block context
	blockContext, err := createBlockContext(ctx, a.tracer, a.metadataManager, parentBlock.View, height, timestamp, len(txs) == 0, rules)
	if err != nil {
		return nil, err
	}

	results, ts, err := a.executeTxs(ctx, timestamp, txs, parentBlock.View, blockContext.feeManager, rules)
	if err != nil {
		return nil, err
	}

	// write the block context
	tsv := ts.NewView(
		state.CompletePermissions,
		state.ImmutableStorage(map[string][]byte{}),
		0,
	)
	if err := writeBlockContext(ctx, a.metadataManager, tsv, blockContext); err != nil {
		return nil, err
	}
	tsv.Commit()

	a.metrics.stateChanges.Add(float64(ts.PendingChanges()))
	a.metrics.stateOperations.Add(float64(ts.OpIndex()))

	view, err := createView(ctx, a.tracer, parentBlock.View, ts.ChangedKeys())
	if err != nil {
		return nil, err
	}

	parentRoot, err := parentBlock.View.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}

	block, err := NewStatelessBlock(
		parentBlock.id,
		timestamp,
		height,
		txs,
		parentRoot,
		pChainBlockContext,
	)
	b := NewExecutionBlock(block) // TODO: clean up separation of block types

	// Kickoff root generation
	go func() {
		start := time.Now()
		root, err := view.GetMerkleRoot(ctx)
		if err != nil {
			a.log.Error("merkle root generation failed", zap.Error(err))
			return
		}
		a.log.Info("merkle root generated",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.id),
			zap.Stringer("root", root),
		)
		a.metrics.rootCalculatedCount.Inc()
		a.metrics.rootCalculatedSum.Add(float64(time.Since(start)))
	}()

	return &OutputBlock{
		ExecutionBlock: b,
		View:           view,
		ExecutionResults: &ExecutionResults{
			Results:       results,
			UnitPrices:    blockContext.feeManager.UnitPrices(),
			UnitsConsumed: blockContext.feeManager.UnitsConsumed(),
		},
	}, nil
}

// collectTxs collects the transactions from the provided chunks and returns a de-duplicated
// slice of transactions for execution.
func (a *Assembler) collectTxs(
	ctx context.Context,
	parentBlock *OutputBlock,
	timestamp int64,
	chunks []*dsmr.Chunk,
) ([]*Transaction, error) {
	// Parse the chunks
	txBatches := make([][]*Transaction, len(chunks))
	totalTxs := 0
	for i, chunk := range chunks {
		txBatch, err := a.batchedTxParser.Unmarshal(chunk.Data)
		if err != nil {
			// TODO: add metrics
			a.log.Warn("Dropping invalid chunk from block",
				zap.Stringer("chunk", chunk),
				zap.Error(err),
			)
		}

		// TODO: perform partition check here or prior to chunk signing
		// TODO: perform signature verification here or prior to chunk signing
		txBatches[i] = txBatch
		totalTxs += len(txBatch)
	}

	// Collect/de-duplicate transactions within the block
	collectedTxs := make([]*Transaction, 0, totalTxs)
	txSet := set.NewSet[ids.ID](totalTxs)
	for _, txBatch := range txBatches {
		for _, tx := range txBatch {
			if txSet.Contains(tx.GetID()) {
				continue
			}
			txSet.Add(tx.GetID())
			collectedTxs = append(collectedTxs, tx)
		}
	}

	// De-duplicate across the validity window and drop txs that cannot pay fees
	dup, err := a.validityWindow.IsRepeat(ctx, parentBlock.ExecutionBlock, timestamp, collectedTxs)
	if err != nil {
		return nil, fmt.Errorf("failed to check for repeats: %w", err)
	}
	txs := make([]*Transaction, 0, len(collectedTxs))
	for i, tx := range collectedTxs {
		if dup.Contains(i) {
			continue
		}

		// TODO: check transactions can pay fees before execution
		// TODO: How to avoid surpassing the MDF limit?
		// 1. Verify chunk limits pre-signing and verify MDF sum during block verification
		// 2. Defer to verify / drop excess transactions post-accept
		// 3. SAE???
		txs = append(txs, tx)
	}

	return txs, nil
}

func (a *Assembler) executeTxs(
	ctx context.Context,
	timestamp int64,
	txs []*Transaction,
	im state.Immutable,
	feeManager *fees.Manager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := a.tracer.Start(ctx, "Assembler.executeTxs")
	defer span.End()

	var (
		numTxs = len(txs)

		f       = fetcher.New(im, numTxs, a.config.StateFetchConcurrency)
		e       = executor.New(numTxs, a.config.TransactionExecutionCores, MaxKeyDependencies, a.metrics.executorVerifyRecorder)
		ts      = tstate.New(numTxs * 2) // TODO: tune this heuristic
		results = make([]*Result, numTxs)
	)

	// Fetch required keys and execute transactions
	for li, ltx := range txs {
		i := li
		tx := ltx

		stateKeys, err := tx.StateKeys(a.balanceHandler)
		if err != nil {
			f.Stop()
			e.Stop()
			return nil, nil, err
		}

		// Ensure we don't consume too many units
		units, err := tx.Units(a.balanceHandler, r)
		if err != nil {
			f.Stop()
			e.Stop()
			return nil, nil, err
		}
		if ok, d := feeManager.Consume(units, r.GetMaxBlockUnits()); !ok {
			f.Stop()
			e.Stop()
			return nil, nil, fmt.Errorf("%w: %d too large", ErrInvalidUnitsConsumed, d)
		}

		// Prefetch state keys from disk
		txID := tx.GetID()
		if err := f.Fetch(ctx, txID, stateKeys.WithoutPermissions()); err != nil {
			return nil, nil, err
		}
		e.Run(stateKeys, func() error {
			// Wait for stateKeys to be read from disk
			storage, err := f.Get(txID)
			if err != nil {
				return err
			}

			// Execute transaction
			//
			// It is critical we explicitly set the scope before each transaction is
			// processed
			tsv := ts.NewView(
				stateKeys,
				state.ImmutableStorage(storage),
				len(stateKeys),
			)

			// Ensure we have enough funds to pay fees
			if err := tx.PreExecute(ctx, feeManager, a.balanceHandler, r, tsv, timestamp); err != nil {
				// TODO: find a better way to handle transactions that fail to pay fees or
				// guarantee this is unreachable, so we can treat it as fatal.
				results[i] = &Result{
					Success: false,
					Error:   []byte(err.Error()),
					Units:   units,
					Fee:     0, // Fee = 0 indicates a dropped transaction
				}
				return err
			}

			result, err := tx.Execute(ctx, feeManager, a.balanceHandler, r, tsv, timestamp)
			if err != nil {
				return err
			}
			results[i] = result

			// Commit results to parent [TState]
			tsv.Commit()
			return nil
		})
	}
	if err := f.Wait(); err != nil {
		return nil, nil, err
	}
	if err := e.Wait(); err != nil {
		return nil, nil, err
	}

	a.metrics.txsVerified.Add(float64(numTxs))

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}
