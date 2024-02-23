// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/fetcher"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

func (b *StatelessBlock) Execute(
	ctx context.Context,
	tracer trace.Tracer, //nolint:interfacer
	im state.Immutable,
	feeManager *fees.Manager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		sm        = b.vm.StateManager()
		numTxs    = len(b.Txs)
		t         = b.GetTimestamp()
		fetchLock sync.RWMutex
		f         = fetcher.New(numTxs, b.vm.GetKeyStorageConcurrency(), im)

		e       = executor.New(numTxs, b.vm.GetTransactionExecutionCores(), b.vm.GetExecutorVerifyRecorder())
		ts      = tstate.New(numTxs * 2) // TODO: tune this heuristic
		results = make([]*Result, numTxs)
	)

	// Fetch required keys and execute transactions
	for li, ltx := range b.Txs {
		i := li
		tx := ltx

		stateKeys, err := tx.StateKeys(sm)
		if err != nil {
			e.Stop()
			return nil, nil, err
		}
		// Fetch keys from disk
		f.Lookup(ctx, tx.ID(), stateKeys)

		e.Run(stateKeys, func() error {
			// Block until worker finishes fetching from disk
			f.TxnsToFetch[tx.ID()].Wait()

			// Fetch keys from cache
			var (
				reads   = make(map[string]uint16, len(stateKeys))
				storage = make(map[string][]byte, len(stateKeys))
			)
			fetchLock.RLock()
			for k := range stateKeys {
				if v, ok := f.Cache[k]; ok {
					reads[k] = v.Chunks
					if v.Exists {
						storage[k] = v.Val
					}
				}
			}
			fetchLock.RUnlock()

			// Execute transaction
			//
			// It is critical we explicitly set the scope before each transaction is
			// processed
			tsv := ts.NewView(stateKeys, storage)

			// Ensure we have enough funds to pay fees
			if err := tx.PreExecute(ctx, feeManager, sm, r, tsv, t); err != nil {
				return err
			}

			// Wait to execute transaction until we have the warp result processed.
			var warpVerified bool
			warpMsg, ok := b.warpMessages[tx.ID()]
			if ok {
				select {
				case warpVerified = <-warpMsg.verifiedChan:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			result, err := tx.Execute(ctx, feeManager, reads, sm, r, tsv, t, ok && warpVerified)
			if err != nil {
				return err
			}
			results[i] = result

			// Update block metadata with units actually consumed (if more is consumed than block allows, we will non-deterministically
			// exit with an error based on which tx over the limit is processed first)
			if ok, d := feeManager.Consume(result.Consumed, r.GetMaxBlockUnits()); !ok {
				return fmt.Errorf("%w: %d too large", ErrInvalidUnitsConsumed, d)
			}

			// Commit results to parent [TState]
			tsv.Commit()
			return nil
		})
	}
	if err := e.Wait(); err != nil {
		return nil, nil, err
	}

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}
