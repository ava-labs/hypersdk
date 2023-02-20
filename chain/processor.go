// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/tstate"
)

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
	go func() {
		defer span.End()

		// Store required keys for each set
		alreadyFetched := map[string]struct{}{}
		for _, tx := range p.blk.GetTxs() {
			storage := map[string][]byte{}
			for _, k := range tx.StateKeys() {
				sk := string(k)
				_, ok := alreadyFetched[sk]
				if ok {
					continue
				}
				v, err := db.GetValue(ctx, k)
				if errors.Is(err, database.ErrNotFound) {
					continue
				} else if err != nil {
					panic(err)
				}
				alreadyFetched[sk] = struct{}{}
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
) (uint64, uint64, []*Result, error) {
	ctx, span := p.tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		unitsConsumed = uint64(0)
		surplusFee    = uint64(0)
		ts            = tstate.New(len(p.blk.Txs)*2, len(p.blk.Txs)*2) // TODO: tune this heuristic
		t             = p.blk.GetTimestamp()
		blkUnitPrice  = p.blk.GetUnitPrice()
		results       = []*Result{}
	)
	for txData := range p.readyTxs {
		tx := txData.tx

		// Update ts
		for k, v := range txData.storage {
			ts.SetStorage(ctx, []byte(k), v)
		}
		// It is critical we explicitly set the scope before each transaction is
		// processed
		ts.SetScope(ctx, tx.StateKeys())

		// Execute tx
		if err := tx.PreExecute(ctx, ectx, r, ts, t); err != nil {
			return 0, 0, nil, err
		}
		result, err := tx.Execute(ctx, r, ts, t)
		if err != nil {
			return 0, 0, nil, err
		}
		surplusFee += (tx.Base.UnitPrice - blkUnitPrice) * result.Units
		results = append(results, result)

		// Update block metadata
		unitsConsumed += result.Units
		if unitsConsumed > r.GetMaxBlockUnits() {
			// Exit as soon as we hit our max
			return 0, 0, nil, ErrBlockTooBig
		}
	}
	// Wait until end to write changes to avoid conflicting with pre-fetching
	return unitsConsumed, surplusFee, results, ts.WriteChanges(ctx, p.db, p.tracer)
}
