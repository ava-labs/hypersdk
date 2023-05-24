// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/tstate"
)

type fetchData struct {
	v      []byte
	exists bool
}

type txData struct {
	tx      *Transaction
	storage map[string][]byte
}

type Processor struct {
	tracer trace.Tracer
	db     merkledb.TrieView
	ts     *tstate.TState

	alreadyFetched map[string]*fetchData

	blk         *StatelessTxBlock
	readyTxs    chan *txData
	prefetchErr error
}

// Only prepare for population if above last accepted height
func NewProcessor(tracer trace.Tracer, db merkledb.TrieView, ts *tstate.TState) *Processor {
	return &Processor{
		tracer:         tracer,
		db:             db,
		ts:             ts,
		alreadyFetched: map[string]*fetchData{},
	}
}

func (p *Processor) Prefetch(ctx context.Context, b *StatelessTxBlock) {
	ctx, span := p.tracer.Start(ctx, "Processor.Prefetch")
	p.blk = b
	sm := p.blk.vm.StateManager()
	p.readyTxs = make(chan *txData, len(p.blk.GetTxs())) // clear from last run
	p.prefetchErr = nil
	go func() {
		defer span.End()

		// Store required keys for each set
		for _, tx := range p.blk.GetTxs() {
			storage := map[string][]byte{}
			for _, k := range tx.StateKeys(sm) {
				sk := string(k)
				if v, ok := p.alreadyFetched[sk]; ok {
					if v.exists {
						storage[sk] = v.v
					}
					continue
				}
				v, err := p.db.GetValue(ctx, k)
				if errors.Is(err, database.ErrNotFound) {
					p.alreadyFetched[sk] = &fetchData{nil, false}
					continue
				} else if err != nil {
					// This can happen if the underlying view changes (if we are
					// verifying a block that can never be accepted).
					p.prefetchErr = err
					close(p.readyTxs)
					return
				}
				p.alreadyFetched[sk] = &fetchData{v, true}
				storage[sk] = v
			}
			p.readyTxs <- &txData{tx, storage}
		}

		// Let caller know all sets have been readied
		close(p.readyTxs)
	}()
}

func (p *Processor) Execute(
	ctx context.Context,
	ectx *TxExecutionContext,
	r Rules,
) (uint64, []*Result, int, int, error) {
	ctx, span := p.tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		unitsConsumed = uint64(0)
		t             = p.blk.GetTimestamp()
		results       = []*Result{}
		sm            = p.blk.vm.StateManager()
		pending       = p.ts.PendingChanges()
		opIndex       = p.ts.OpIndex()
	)
	for txData := range p.readyTxs {
		tx := txData.tx

		// It is critical we explicitly set the scope before each transaction is
		// processed
		p.ts.SetScope(ctx, tx.StateKeys(sm), txData.storage)

		// Execute tx
		if err := tx.PreExecute(ctx, ectx, r, p.ts, t); err != nil {
			return 0, nil, 0, 0, err
		}
		// Wait to execute transaction until we have the warp result processed.
		//
		// TODO: parallel execution will greatly improve performance in the case
		// that we are waiting for signature verification.
		var warpVerified bool
		warpMsg, ok := p.blk.warpMessages[tx.ID()]
		if ok {
			select {
			case warpVerified = <-warpMsg.verifiedChan:
			case <-ctx.Done():
				return 0, nil, 0, 0, ctx.Err()
			}
		}
		result, err := tx.Execute(ctx, ectx, r, sm, p.ts, t, ok && warpVerified)
		if err != nil {
			return 0, nil, 0, 0, err
		}
		results = append(results, result)

		// Update block metadata
		unitsConsumed += result.Units
		if unitsConsumed > r.GetMaxTxBlockUnits() {
			// Exit as soon as we hit our max
			return 0, nil, 0, 0, ErrBlockTooBig
		}
	}
	return unitsConsumed, results, p.ts.PendingChanges() - pending, p.ts.OpIndex() - opIndex, p.prefetchErr
}

// Wait until end to write changes to avoid conflicting with pre-fetching
func (p *Processor) Commit(ctx context.Context) error {
	return p.ts.WriteChanges(ctx, p.db, p.tracer)
}
