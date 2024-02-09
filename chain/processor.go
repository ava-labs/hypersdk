// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/keys"
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
		cacheLock sync.RWMutex
		cache     = make(map[string]*fetchData, numTxs)

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
		e.Run(stateKeys, func() error {
			// Fetch keys from cache
			var (
				reads    = make(map[string]uint16, len(stateKeys))
				storage  = make(map[string][]byte, len(stateKeys))
				toLookup = make([]string, 0, len(stateKeys))
			)
			cacheLock.RLock()
			for k := range stateKeys {
				if v, ok := cache[k]; ok {
					reads[k] = v.chunks
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
					v, err := im.GetValue(ctx, []byte(k))
					if errors.Is(err, database.ErrNotFound) {
						reads[k] = 0
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
					reads[k] = numChunks
					toCache[k] = &fetchData{v, true, numChunks}
					storage[k] = v
				}
			}

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

			// Update key cache
			if len(toCache) > 0 {
				cacheLock.Lock()
				for k := range toCache {
					cache[k] = toCache[k]
				}
				cacheLock.Unlock()
			}
			return nil
		})
	}
	if err := e.Wait(); err != nil {
		return nil, nil, err
	}

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}
