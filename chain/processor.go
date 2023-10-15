// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/executor"
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
	tracer trace.Tracer,
	im state.Immutable,
	feeManager *FeeManager,
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

		e       = executor.New(numTxs, 8) // TODO: make concurrency configurable
		ts      = tstate.New(numTxs * 2)  // TODO: tune this heuristic
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
			cacheLock.RLock()
			var (
				coldReads = map[string]uint16{}
				warmReads = map[string]uint16{}
				storage   = map[string][]byte{}
				toLookup  = make([]string, 0, len(stateKeys))
			)
			for k := range stateKeys {
				if v, ok := cache[k]; ok {
					warmReads[k] = v.chunks
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
						coldReads[k] = 0
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
					coldReads[k] = numChunks
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
			authCUs, err := tx.PreExecute(ctx, feeManager, sm, r, tsv, t)
			if err != nil {
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
			result, err := tx.Execute(ctx, feeManager, authCUs, coldReads, warmReads, sm, r, tsv, t, ok && warpVerified)
			if err != nil {
				return err
			}
			results[i] = result

			// Commit results to parent [TState]
			tsv.Commit()

			// Update block metadata with units actually consumed (if more is consumed than block allows, we will non-deterministically
			// exit with an error based on which tx over the limit is processed first)
			if err := feeManager.Consume(result.Consumed); err != nil {
				return err
			}

			// Update key cache
			if len(toCache) > 0 {
				cacheLock.Lock()
				for k := range toCache {
					fmt.Println("adding key to cache", hex.EncodeToString([]byte(k)))
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
