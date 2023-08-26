// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

type txData struct {
	tx      *Transaction
	storage map[string][]byte

	coldReads map[string]uint16
	warmReads map[string]uint16
}

type Processor struct {
	tracer trace.Tracer

	err      error
	blk      *StatelessBlock
	readyTxs chan *txData
	im       state.Immutable
}

// Only prepare for population if above last accepted height
func NewProcessor(tracer trace.Tracer, b *StatelessBlock) *Processor {
	return &Processor{
		tracer: tracer,

		blk:      b,
		readyTxs: make(chan *txData, len(b.GetTxs())),
	}
}

func (p *Processor) Prefetch(ctx context.Context, im state.Immutable) {
	ctx, span := p.tracer.Start(ctx, "Processor.Prefetch")
	p.im = im
	sm := p.blk.vm.StateManager()
	go func() {
		defer func() {
			close(p.readyTxs) // let caller know all sets have been readied
			span.End()
		}()

		// Store required keys for each set
		alreadyFetched := make(map[string]*fetchData, len(p.blk.GetTxs()))
		for _, tx := range p.blk.GetTxs() {
			coldReads := map[string]uint16{}
			warmReads := map[string]uint16{}
			storage := map[string][]byte{}
			stateKeys, err := tx.StateKeys(sm)
			if err != nil {
				p.err = err
				return
			}
			for k := range stateKeys {
				if v, ok := alreadyFetched[k]; ok {
					warmReads[k] = v.chunks
					if v.exists {
						storage[k] = v.v
					}
					continue
				}
				v, err := im.GetValue(ctx, []byte(k))
				if errors.Is(err, database.ErrNotFound) {
					coldReads[k] = 0
					alreadyFetched[k] = &fetchData{nil, false, 0}
					continue
				} else if err != nil {
					p.err = err
					return
				}
				// We verify that the [NumChunks] is already less than the number
				// added on the write path, so we don't need to do so again here.
				numChunks, ok := keys.NumChunks(v)
				if !ok {
					p.err = ErrInvalidKeyValue
					return
				}
				coldReads[k] = numChunks
				alreadyFetched[k] = &fetchData{v, true, numChunks}
				storage[k] = v
			}
			p.readyTxs <- &txData{tx, storage, coldReads, warmReads}
		}
	}()
}

func (p *Processor) Execute(
	ctx context.Context,
	feeManager *FeeManager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := p.tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		ts      = tstate.New(len(p.blk.Txs) * 2) // TODO: tune this heuristic
		t       = p.blk.GetTimestamp()
		results = []*Result{}
		sm      = p.blk.vm.StateManager()
	)
	for txData := range p.readyTxs {
		if p.err != nil {
			return nil, nil, p.err
		}

		tx := txData.tx

		// Ensure can process next tx
		nextUnits, err := tx.MaxUnits(sm, r)
		if err != nil {
			return nil, nil, err
		}
		if ok, dimension := feeManager.CanConsume(nextUnits, r.GetMaxBlockUnits()); !ok {
			return nil, nil, fmt.Errorf("dimension %d exceeds limit", dimension)
		}

		// It is critical we explicitly set the scope before each transaction is
		// processed
		stateKeys, err := tx.StateKeys(sm)
		if err != nil {
			return nil, nil, err
		}
		ts.SetScope(ctx, stateKeys, txData.storage)

		// Execute tx
		authCUs, err := tx.PreExecute(ctx, feeManager, sm, r, ts, t)
		if err != nil {
			return nil, nil, err
		}
		// Wait to execute transaction until we have the warp result processed.
		//
		// TODO: parallel execution will greatly improve performance when actions
		// start taking longer than a few ns (i.e. with hypersdk programs).
		var warpVerified bool
		warpMsg, ok := p.blk.warpMessages[tx.ID()]
		if ok {
			select {
			case warpVerified = <-warpMsg.verifiedChan:
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		result, err := tx.Execute(ctx, feeManager, authCUs, txData.coldReads, txData.warmReads, sm, r, ts, t, ok && warpVerified)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, result)

		// Update block metadata with units actually consumed
		if err := feeManager.Consume(result.Consumed); err != nil {
			return nil, nil, err
		}
	}
	// Wait until end to write changes to avoid conflicting with pre-fetching
	if p.err != nil {
		return nil, nil, p.err
	}
	return results, ts, nil
}
