// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/AnomalyFi/hypersdk/tstate"
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
			for _, k := range tx.StateKeys(sm) {
				sk := string(k)
				if v, ok := alreadyFetched[sk]; ok {
					if v.exists {
						storage[sk] = v.v
					}
					continue
				}
				v, err := db.GetValue(ctx, k)
				if errors.Is(err, database.ErrNotFound) {
					alreadyFetched[sk] = &fetchData{nil, false}
					continue
				} else if err != nil {
					panic(err)
				}
				alreadyFetched[sk] = &fetchData{v, true}
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
	ectx *ExecutionContext,
	r Rules,
) (uint64, uint64, []*Result, int, int, error) {
	ctx, span := p.tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		unitsConsumed = uint64(0)
		surplusFee    = uint64(0)
		ts            = tstate.New(len(p.blk.Txs) * 2) // TODO: tune this heuristic
		t             = p.blk.GetTimestamp()
		blkUnitPrice  = p.blk.GetUnitPrice()
		results       = []*Result{}
		sm            = p.blk.vm.StateManager()
	)
	for txData := range p.readyTxs {
		tx := txData.tx

		// It is critical we explicitly set the scope before each transaction is
		// processed
		ts.SetScope(ctx, tx.StateKeys(sm), txData.storage)

		// Execute tx
		if err := tx.PreExecute(ctx, ectx, r, ts, t); err != nil {
			return 0, 0, nil, 0, 0, err
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
				return 0, 0, nil, 0, 0, ctx.Err()
			}
		}
		result, err := tx.Execute(ctx, ectx, r, sm, ts, t, ok && warpVerified)
		if err != nil {
			return 0, 0, nil, 0, 0, err
		}
		surplusFee += (tx.Base.UnitPrice - blkUnitPrice) * result.Units
		results = append(results, result)

		// Update block metadata
		unitsConsumed += result.Units
		if unitsConsumed > r.GetMaxBlockUnits() {
			// Exit as soon as we hit our max
			return 0, 0, nil, 0, 0, ErrBlockTooBig
		}
	}
	// Wait until end to write changes to avoid conflicting with pre-fetching
	if err := ts.WriteChanges(ctx, p.db, p.tracer); err != nil {
		return 0, 0, nil, 0, 0, err
	}
	return unitsConsumed, surplusFee, results, ts.PendingChanges(), ts.OpIndex(), nil
}
