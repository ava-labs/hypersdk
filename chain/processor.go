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

	parentState Database

	blk      *StatelessBlock
	readyTxs chan *txData

	err error
}

// Only prepare for population if above last accepted height
func NewProcessor(tracer trace.Tracer, b *StatelessBlock) *Processor {
	return &Processor{
		tracer: tracer,

		blk:      b,
		readyTxs: make(chan *txData, len(b.GetTxs())),
	}
}

func (p *Processor) Prefetch(ctx context.Context, parentState Database) {
	ctx, span := p.tracer.Start(ctx, "Processor.Prefetch")
	p.parentState = parentState
	sm := p.blk.vm.StateManager()
	go func() {
		defer func() {
			// Let caller know all sets have been readied
			close(p.readyTxs)
			span.End()
		}()

		// Store required keys for each set
		alreadyFetched := map[string][]byte{}
		for _, tx := range p.blk.GetTxs() {
			storage := map[string][]byte{}
			for _, k := range tx.StateKeys(sm) {
				sk := string(k)
				if v, ok := alreadyFetched[sk]; ok {
					storage[sk] = v
					continue
				}
				v, err := p.parentState.GetValue(ctx, k)
				if errors.Is(err, database.ErrNotFound) {
					continue
				} else if err != nil {
					// This can happen if a conflicting ancestry of the underlying merkledb
					// is committed.
					p.err = err
					return
				}
				alreadyFetched[sk] = v
				storage[sk] = v
			}
			p.readyTxs <- &txData{tx, storage}
		}
	}()
}

func (p *Processor) Execute(
	ctx context.Context,
	newState Database,
	ectx *ExecutionContext,
	r Rules,
) (uint64, uint64, []*Result, error) {
	ctx, span := p.tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		unitsConsumed = uint64(0)
		surplusFee    = uint64(0)
		ts            = tstate.New(newState, statePreallocation) // TODO: tune this heuristic
		t             = p.blk.GetTimestamp()
		blkUnitPrice  = p.blk.GetUnitPrice()
		results       = []*Result{}
		sm            = p.blk.vm.StateManager()
	)
	for txData := range p.readyTxs {
		tx := txData.tx

		// It is critical we explicitly set the scope before each transaction is
		// processed.
		//
		// Any storage that has been set in previous transactions will be used
		// instead of the prefetched state provided here.
		ts.SetScope(ctx, tx.StateKeys(sm), txData.storage)

		// Execute tx
		if err := tx.PreExecute(ctx, ectx, r, ts, t); err != nil {
			return 0, 0, nil, err
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
				return 0, 0, nil, ctx.Err()
			}
		}
		result, err := tx.Execute(ctx, ectx, r, sm, ts, t, ok && warpVerified)
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
	if p.err != nil {
		return 0, 0, nil, p.err
	}
	// Wait until end to write changes to avoid conflicting with pre-fetching
	return unitsConsumed, surplusFee, results, nil
}
