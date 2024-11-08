// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/fetcher"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func (c *Chain) Execute(
	ctx context.Context,
	parentView state.View,
	b *ExecutionBlock,
) (*ExecutedBlock, merkledb.View, error) {
	ctx, span := c.tracer.Start(ctx, "Chain.Execute")
	defer span.End()

	var (
		r   = c.ruleFactory.GetRules(b.Tmstmp)
		log = c.log
	)

	// Perform basic correctness checks before doing any expensive work
	if b.Tmstmp > time.Now().Add(FutureBound).UnixMilli() {
		return nil, nil, ErrTimestampTooLate
	}
	// AsyncVerify should have been called already. We call it here defensively.
	if err := c.AsyncVerify(ctx, b); err != nil {
		return nil, nil, err
	}

	// Fetch parent height key and ensure block height is valid
	heightKey := HeightKey(c.metadataManager.HeightPrefix())
	parentHeightRaw, err := parentView.GetValue(ctx, heightKey)
	if err != nil {
		return nil, nil, err
	}
	parentHeight, err := database.ParseUInt64(parentHeightRaw)
	if err != nil {
		return nil, nil, err
	}
	if b.Hght != parentHeight+1 {
		return nil, nil, ErrInvalidBlockHeight
	}

	// Fetch parent timestamp and confirm block timestamp is valid
	//
	// Parent may not be available (if we preformed state sync), so we
	// can't rely on being able to fetch it during verification.
	timestampKey := TimestampKey(c.metadataManager.TimestampPrefix())
	parentTimestampRaw, err := parentView.GetValue(ctx, timestampKey)
	if err != nil {
		return nil, nil, err
	}
	parentTimestampUint64, err := database.ParseUInt64(parentTimestampRaw)
	if err != nil {
		return nil, nil, err
	}
	parentTimestamp := int64(parentTimestampUint64)
	if b.Tmstmp < parentTimestamp+r.GetMinBlockGap() {
		return nil, nil, ErrTimestampTooEarly
	}
	if len(b.Txs) == 0 && b.Tmstmp < parentTimestamp+r.GetMinEmptyBlockGap() {
		return nil, nil, ErrTimestampTooEarly
	}

	if err := c.validityWindow.VerifyExpiryReplayProtection(ctx, b, parentTimestamp); err != nil {
		return nil, nil, err
	}

	// Compute next unit prices to use
	feeKey := FeeKey(c.metadataManager.FeePrefix())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return nil, nil, err
	}
	parentFeeManager := fees.NewManager(feeRaw)
	feeManager, err := parentFeeManager.ComputeNext(b.Tmstmp, r)
	if err != nil {
		return nil, nil, err
	}

	// Process transactions
	results, ts, err := c.executeTxs(ctx, b, parentView, feeManager, r)
	if err != nil {
		log.Error("failed to execute block", zap.Error(err))
		return nil, nil, err
	}

	// Update chain metadata
	heightKeyStr := string(heightKey)
	timestampKeyStr := string(timestampKey)
	feeKeyStr := string(feeKey)

	keys := make(state.Keys)
	keys.Add(heightKeyStr, state.Write)
	keys.Add(timestampKeyStr, state.Write)
	keys.Add(feeKeyStr, state.Write)
	tsv := ts.NewView(keys, map[string][]byte{
		heightKeyStr:    parentHeightRaw,
		timestampKeyStr: parentTimestampRaw,
		feeKeyStr:       parentFeeManager.Bytes(),
	})
	if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, nil, err
	}
	if err := tsv.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(b.Tmstmp))); err != nil {
		return nil, nil, err
	}
	if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
		return nil, nil, err
	}
	tsv.Commit()

	// Compare state root
	//
	// Because fee bytes are not recorded in state, it is sufficient to check the state root
	// to verify all fee calculations were correct.
	_, rspan := c.tracer.Start(ctx, "Chain.Execute.WaitRoot")
	start := time.Now()
	computedRoot, err := parentView.GetMerkleRoot(ctx)
	rspan.End()
	if err != nil {
		return nil, nil, err
	}
	c.metrics.waitRoot.Observe(float64(time.Since(start)))
	if b.StateRoot != computedRoot {
		return nil, nil, fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			b.StateRoot,
		)
	}

	// Ensure signatures are verified
	_, sspan := c.tracer.Start(ctx, "Chain.Execute.WaitSignatures")
	start = time.Now()
	err = b.sigJob.Wait()
	sspan.End()
	if err != nil {
		return nil, nil, err
	}
	c.metrics.waitSignatures.Observe(float64(time.Since(start)))

	// Get view from [tstate] after processing all state transitions
	c.metrics.stateChanges.Add(float64(ts.PendingChanges()))
	c.metrics.stateOperations.Add(float64(ts.OpIndex()))
	view, err := ts.ExportMerkleDBView(ctx, c.tracer, parentView)
	if err != nil {
		return nil, nil, err
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
			zap.Stringer("blkID", b.id),
			zap.Stringer("root", root),
		)
		c.metrics.rootCalculated.Observe(float64(time.Since(start)))
	}()

	return &ExecutedBlock{
		Block:         b.StatelessBlock,
		Results:       results,
		UnitPrices:    feeManager.UnitPrices(),
		UnitsConsumed: feeManager.UnitsConsumed(),
	}, view, nil
}

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

func (c *Chain) executeTxs(
	ctx context.Context,
	b *ExecutionBlock,
	im state.Immutable,
	feeManager *fees.Manager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := c.tracer.Start(ctx, "Chain.Execute.ExecuteTxs")
	defer span.End()

	var (
		numTxs = len(b.Txs)
		t      = b.Tmstmp

		f       = fetcher.New(im, numTxs, c.config.StateFetchConcurrency)
		e       = executor.New(numTxs, c.config.TransactionExecutionCores, MaxKeyDependencies, c.metrics.executorVerifyRecorder)
		ts      = tstate.New(numTxs * 2) // TODO: tune this heuristic
		results = make([]*Result, numTxs)
	)

	// Fetch required keys and execute transactions
	for li, ltx := range b.Txs {
		i := li
		tx := ltx

		stateKeys, err := tx.StateKeys(c.balanceHandler)
		if err != nil {
			f.Stop()
			e.Stop()
			return nil, nil, err
		}

		// Ensure we don't consume too many units
		units, err := tx.Units(c.balanceHandler, r)
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
		txID := tx.ID()
		if err := f.Fetch(ctx, txID, stateKeys); err != nil {
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
			tsv := ts.NewView(stateKeys, storage)

			// Ensure we have enough funds to pay fees
			if err := tx.PreExecute(ctx, feeManager, c.balanceHandler, r, tsv, t); err != nil {
				return err
			}

			result, err := tx.Execute(ctx, feeManager, c.balanceHandler, r, tsv, t)
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

	c.metrics.txsVerified.Add(float64(len(b.Txs)))

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}
