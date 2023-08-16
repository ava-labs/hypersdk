// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"

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

	blk      *StatelessBlock
	readyTxs chan *txData
	db       Database
}

// Only prepare for population if above last accepted height
func NewProcessor(tracer trace.Tracer, b *StatelessBlock) *Processor {
	return &Processor{
		tracer: tracer,

		blk:      b,
		readyTxs: make(chan *txData, len(b.GetTxs())),
	}
}

func (p *Processor) Prefetch(ctx context.Context, db Database) {
	ctx, span := p.tracer.Start(ctx, "Processor.Prefetch")
	p.db = db
	sm := p.blk.vm.StateManager()
	go func() {
		defer span.End()

		// Store required keys for each set
		alreadyFetched := make(map[string]*fetchData, len(p.blk.GetTxs()))
		for _, tx := range p.blk.GetTxs() {
			storage := map[string][]byte{}
			for k := range tx.StateKeys(sm) {
				if v, ok := alreadyFetched[k]; ok {
					if v.exists {
						storage[k] = v.v
					}
					continue
				}
				v, err := db.GetValue(ctx, []byte(k))
				if errors.Is(err, database.ErrNotFound) {
					alreadyFetched[k] = &fetchData{nil, false}
					continue
				} else if err != nil {
					panic(err)
				}
				alreadyFetched[k] = &fetchData{v, true}
				storage[k] = v
			}
			p.readyTxs <- &txData{tx, storage}
		}

		// Let caller know all sets have been readied
		close(p.readyTxs)
	}()
}

func (p *Processor) Execute(
	ctx context.Context,
	feeManager *FeeManager,
	r Rules,
) ([]*Result, int, int, error) {
	ctx, span := p.tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		ts      = tstate.New(len(p.blk.Txs) * 2) // TODO: tune this heuristic
		t       = p.blk.GetTimestamp()
		results = []*Result{}
		sm      = p.blk.vm.StateManager()
	)
	for txData := range p.readyTxs {
		tx := txData.tx

		// Ensure can process next tx
		nextUnits, err := tx.MaxUnits(sm, r)
		if err != nil {
			return nil, 0, 0, err
		}
		if ok, dimension := feeManager.CanConsume(nextUnits, r.GetMaxBlockUnits()); !ok {
			return nil, 0, 0, fmt.Errorf("dimension %d exceeds limit", dimension)
		}

		// It is critical we explicitly set the scope before each transaction is
		// processed
		ts.SetScope(ctx, tx.StateKeys(sm), txData.storage)

		// Execute tx
		if err := tx.PreExecute(ctx, feeManager, sm, r, ts, t); err != nil {
			return nil, 0, 0, err
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
				return nil, 0, 0, ctx.Err()
			}
		}
		result, err := tx.Execute(ctx, feeManager, sm, r, ts, t, ok && warpVerified)
		if err != nil {
			return nil, 0, 0, err
		}
		results = append(results, result)

		// Update block metadata
		if err := feeManager.Consume(nextUnits); err != nil {
			return nil, 0, 0, err
		}
	}
	// Wait until end to write changes to avoid conflicting with pre-fetching
	if err := ts.WriteChanges(ctx, p.db, p.tracer); err != nil {
		return nil, 0, 0, err
	}
	return results, ts.PendingChanges(), ts.OpIndex(), nil
}
