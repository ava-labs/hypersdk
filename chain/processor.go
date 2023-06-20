// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/tstate"
)

const readyTxBacklog = 8_192

type fetchData struct {
	v      []byte
	exists bool
}

type txData struct {
	tx      *Transaction
	storage map[string][]byte
	skip    bool
}

type Processor struct {
	tracer trace.Tracer
	db     merkledb.TrieView
	ts     *tstate.TState

	alreadyFetched map[string]*fetchData

	blk         *StatelessRootBlock
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

func (p *Processor) Prefetch(ctx context.Context, b *StatelessRootBlock) {
	ctx, span := p.tracer.Start(ctx, "Processor.Prefetch")
	p.blk = b
	sm := p.blk.vm.StateManager()
	p.readyTxs = make(chan *txData, readyTxBacklog) // clear from last run
	p.prefetchErr = nil
	verifySignatues := p.blk.vm.GetVerifySignatures()
	go func() {
		defer span.End()

		// Store required keys for each set
		for _, txBlk := range p.blk.GetTxBlocks() {
			for i, tx := range txBlk.Txs {
				if txBlk.repeats.Contains(i) {
					p.blk.vm.Logger().Warn("tx is duplicate", zap.Stringer("txID", tx.ID()))
					p.readyTxs <- &txData{tx, nil, true}
					continue
				}

				// Check signature before we interact with disk
				if verifySignatues {
					if err := tx.WaitAuthVerified(ctx); err != nil {
						p.blk.vm.Logger().Warn("tx signature is wrong", zap.Stringer("txID", tx.ID()), zap.Error(err))
						p.readyTxs <- &txData{tx, nil, true}
						continue
					}
				}

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
				p.readyTxs <- &txData{tx, storage, false}
			}
		}

		// Let caller know all sets have been readied
		close(p.readyTxs)
	}()
}

func (p *Processor) Execute(
	ctx context.Context,
	ectx *TxExecutionContext,
	r Rules,
	warpMessages map[ids.ID]*warpJob,
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
		logger        = p.blk.vm.Logger()
	)
	for txData := range p.readyTxs {
		tx := txData.tx
		if txData.skip {
			p.blk.vm.RecordTxFailedExecution()
			results = append(results, &Result{})
			continue
		}

		// It is critical we explicitly set the scope before each transaction is
		// processed
		p.ts.SetScope(ctx, tx.StateKeys(sm), txData.storage)

		// Execute tx
		if err := tx.PreExecute(ctx, ectx, r, p.ts, t); err != nil {
			logger.Warn("tx failed execution", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.blk.vm.RecordTxFailedExecution()
			results = append(results, &Result{})
			continue
		}
		// Wait to execute transaction until we have the warp result processed.
		//
		// TODO: parallel execution will greatly improve performance in the case
		// that we are waiting for signature verification.
		//
		// We keep this here because we charge fees regardless of whether the message is successful.
		var warpVerified bool
		warpMsg, ok := warpMessages[tx.ID()]
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
	}
	return unitsConsumed, results, p.ts.PendingChanges() - pending, p.ts.OpIndex() - opIndex, p.prefetchErr
}

// Wait until end to write changes to avoid conflicting with pre-fetching
func (p *Processor) Commit(ctx context.Context) error {
	return p.ts.WriteChanges(ctx, p.db, p.tracer)
}
