// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/fetcher"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

type ExecutionBlock struct {
	*StatelessBlock

	// authCounts can be used by batch signature verification
	// to preallocate memory
	authCounts map[uint8]int
	txsSet     set.Set[ids.ID]
	sigJob     workers.Job
}

func NewExecutionBlock(block *StatelessBlock) (*ExecutionBlock, error) {
	authCounts := make(map[uint8]int)
	txsSet := set.NewSet[ids.ID](len(block.Txs))
	for _, tx := range block.Txs {
		if txsSet.Contains(tx.GetID()) {
			return nil, ErrDuplicateTx
		}
		txsSet.Add(tx.GetID())
		authCounts[tx.Auth.GetTypeID()]++
	}

	return &ExecutionBlock{
		authCounts:     authCounts,
		txsSet:         txsSet,
		StatelessBlock: block,
	}, nil
}

func (b *ExecutionBlock) ContainsTx(id ids.ID) bool {
	return b.txsSet.Contains(id)
}

func (b *ExecutionBlock) Height() uint64 {
	return b.Hght
}

func (b *ExecutionBlock) Parent() ids.ID {
	return b.Prnt
}

func (b *ExecutionBlock) Timestamp() int64 {
	return b.Tmstmp
}

func (b *ExecutionBlock) Txs() []*Transaction {
	return b.StatelessBlock.Txs
}

type Processor struct {
	tracer                  trace.Tracer
	log                     logging.Logger
	ruleFactory             RuleFactory
	authVerificationWorkers workers.Workers
	authVM                  AuthVM
	metadataManager         MetadataManager
	balanceHandler          BalanceHandler
	validityWindow          ValidityWindow
	metrics                 *chainMetrics
	config                  Config
}

func NewProcessor(
	tracer trace.Tracer,
	log logging.Logger,
	ruleFactory RuleFactory,
	authVerificationWorkers workers.Workers,
	authVM AuthVM,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
	validityWindow ValidityWindow,
	metrics *chainMetrics,
	config Config,
) *Processor {
	return &Processor{
		tracer:                  tracer,
		log:                     log,
		ruleFactory:             ruleFactory,
		authVerificationWorkers: authVerificationWorkers,
		authVM:                  authVM,
		metadataManager:         metadataManager,
		balanceHandler:          balanceHandler,
		validityWindow:          validityWindow,
		metrics:                 metrics,
		config:                  config,
	}
}

func (p *Processor) Execute(
	ctx context.Context,
	parentView state.View,
	b *ExecutionBlock,
) (*ExecutedBlock, merkledb.View, error) {
	ctx, span := p.tracer.Start(ctx, "Chain.Execute")
	defer span.End()

	var (
		r   = p.ruleFactory.GetRules(b.Tmstmp)
		log = p.log
	)

	// Perform basic correctness checks before doing any expensive work
	if b.Tmstmp > time.Now().Add(FutureBound).UnixMilli() {
		return nil, nil, ErrTimestampTooLate
	}
	// AsyncVerify should have been called already. We call it here defensively.
	if err := p.AsyncVerify(ctx, b); err != nil {
		return nil, nil, err
	}

	// Fetch parent height key and ensure block height is valid
	heightKey := HeightKey(p.metadataManager.HeightPrefix())
	parentHeightRaw, err := parentView.GetValue(ctx, heightKey)
	if err != nil {
		return nil, nil, err
	}
	parentHeight, err := database.ParseUInt64(parentHeightRaw)
	if err != nil {
		return nil, nil, err
	}
	if b.Hght != parentHeight+1 {
		return nil, nil, ErrInvalidBlockHeight
	}

	// Fetch parent timestamp and confirm block timestamp is valid
	//
	// Parent may not be available (if we preformed state sync), so we
	// can't rely on being able to fetch it during verification.
	timestampKey := TimestampKey(p.metadataManager.TimestampPrefix())
	parentTimestampRaw, err := parentView.GetValue(ctx, timestampKey)
	if err != nil {
		return nil, nil, err
	}
	parentTimestampUint64, err := database.ParseUInt64(parentTimestampRaw)
	if err != nil {
		return nil, nil, err
	}
	parentTimestamp := int64(parentTimestampUint64)
	if b.Tmstmp < parentTimestamp+r.GetMinBlockGap() {
		return nil, nil, ErrTimestampTooEarly
	}
	if len(b.StatelessBlock.Txs) == 0 && b.Tmstmp < parentTimestamp+r.GetMinEmptyBlockGap() {
		return nil, nil, ErrTimestampTooEarly
	}

	if err := p.validityWindow.VerifyExpiryReplayProtection(ctx, b, parentTimestamp); err != nil {
		return nil, nil, err
	}

	// Compute next unit prices to use
	feeKey := FeeKey(p.metadataManager.FeePrefix())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return nil, nil, err
	}
	parentFeeManager := fees.NewManager(feeRaw)
	feeManager, err := parentFeeManager.ComputeNext(b.Tmstmp, r)
	if err != nil {
		return nil, nil, err
	}

	// Process transactions
	results, ts, err := p.executeTxs(ctx, b, parentView, feeManager, r)
	if err != nil {
		log.Error("failed to execute block", zap.Error(err))
		return nil, nil, err
	}

	// Update chain metadata
	heightKeyStr := string(heightKey)
	timestampKeyStr := string(timestampKey)
	feeKeyStr := string(feeKey)

	keys := make(state.Keys)
	keys.Add(heightKeyStr, state.Write)
	keys.Add(timestampKeyStr, state.Write)
	keys.Add(feeKeyStr, state.Write)
	tsv := ts.NewView(
		state.NewDefaultScope(keys),
		state.ImmutableStorage(map[string][]byte{
			heightKeyStr:    parentHeightRaw,
			timestampKeyStr: parentTimestampRaw,
			feeKeyStr:       parentFeeManager.Bytes(),
		}),
		0,
	)
	if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, nil, err
	}
	if err := tsv.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(b.Tmstmp))); err != nil {
		return nil, nil, err
	}
	if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
		return nil, nil, err
	}
	tsv.Commit()

	// Compare state root
	//
	// Because fee bytes are not recorded in state, it is sufficient to check the state root
	// to verify all fee calculations were correct.
	_, rspan := p.tracer.Start(ctx, "Chain.Execute.WaitRoot")
	start := time.Now()
	computedRoot, err := parentView.GetMerkleRoot(ctx)
	rspan.End()
	if err != nil {
		return nil, nil, err
	}
	p.metrics.waitRootCount.Inc()
	p.metrics.waitRootSum.Add(float64(time.Since(start)))
	if b.StateRoot != computedRoot {
		return nil, nil, fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			b.StateRoot,
		)
	}

	// Ensure signatures are verified
	_, sspan := p.tracer.Start(ctx, "Chain.Execute.WaitSignatures")
	start = time.Now()
	err = b.sigJob.Wait()
	sspan.End()
	if err != nil {
		return nil, nil, err
	}
	p.metrics.waitSignaturesCount.Inc()
	p.metrics.waitSignaturesSum.Add(float64(time.Since(start)))

	// Get view from [tstate] after processing all state transitions
	p.metrics.stateChanges.Add(float64(ts.PendingChanges()))
	p.metrics.stateOperations.Add(float64(ts.OpIndex()))
	view, err := ts.ExportMerkleDBView(ctx, p.tracer, parentView)
	if err != nil {
		return nil, nil, err
	}

	// Kickoff root generation
	go func() {
		start := time.Now()
		root, err := view.GetMerkleRoot(ctx)
		if err != nil {
			log.Error("merkle root generation failed", zap.Error(err))
			return
		}
		log.Info("merkle root generated",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.id),
			zap.Stringer("root", root),
		)
		p.metrics.rootCalculatedCount.Inc()
		p.metrics.rootCalculatedSum.Add(float64(time.Since(start)))
	}()

	return &ExecutedBlock{
		Block:         b.StatelessBlock,
		Results:       results,
		UnitPrices:    feeManager.UnitPrices(),
		UnitsConsumed: feeManager.UnitsConsumed(),
	}, view, nil
}

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

func (p *Processor) executeTxs(
	ctx context.Context,
	b *ExecutionBlock,
	im state.Immutable,
	feeManager *fees.Manager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := p.tracer.Start(ctx, "Chain.Execute.ExecuteTxs")
	defer span.End()

	var (
		numTxs = len(b.StatelessBlock.Txs)
		t      = b.Tmstmp

		f       = fetcher.New(im, numTxs, p.config.StateFetchConcurrency)
		e       = executor.New(numTxs, p.config.TransactionExecutionCores, MaxKeyDependencies, p.metrics.executorVerifyRecorder)
		ts      = tstate.New(numTxs * 2) // TODO: tune this heuristic
		results = make([]*Result, numTxs)
	)

	// Fetch required keys and execute transactions
	for li, ltx := range b.StatelessBlock.Txs {
		i := li
		tx := ltx

		stateKeys, err := tx.StateKeys(p.balanceHandler)
		if err != nil {
			f.Stop()
			e.Stop()
			return nil, nil, err
		}

		// Ensure we don't consume too many units
		units, err := tx.Units(p.balanceHandler, r)
		if err != nil {
			f.Stop()
			e.Stop()
			return nil, nil, err
		}
		if ok, d := feeManager.Consume(units, r.GetMaxBlockUnits()); !ok {
			f.Stop()
			e.Stop()
			return nil, nil, fmt.Errorf("%w: %d too large", ErrInvalidUnitsConsumed, d)
		}

		// Prefetch state keys from disk
		txID := tx.GetID()
		if err := f.Fetch(ctx, txID, stateKeys.WithoutPermissions()); err != nil {
			return nil, nil, err
		}
		e.Run(stateKeys, func() error {
			// Wait for stateKeys to be read from disk
			storage, err := f.Get(txID)
			if err != nil {
				return err
			}

			// Execute transaction
			//
			// It is critical we explicitly set the scope before each transaction is
			// processed
			tsv := ts.NewView(
				state.NewDefaultScope(stateKeys),
				state.ImmutableStorage(storage),
				0,
			)

			// Ensure we have enough funds to pay fees
			if err := tx.PreExecute(ctx, feeManager, p.balanceHandler, r, tsv, t); err != nil {
				return err
			}

			result, err := tx.Execute(ctx, feeManager, p.balanceHandler, r, tsv, t)
			if err != nil {
				return err
			}
			results[i] = result

			// Commit results to parent [TState]
			tsv.Commit()
			return nil
		})
	}
	if err := f.Wait(); err != nil {
		return nil, nil, err
	}
	if err := e.Wait(); err != nil {
		return nil, nil, err
	}

	p.metrics.txsVerified.Add(float64(len(b.StatelessBlock.Txs)))

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}

// AsyncVerify starts async signature verification as early as possible
func (p *Processor) AsyncVerify(ctx context.Context, block *ExecutionBlock) error {
	ctx, span := p.tracer.Start(ctx, "Chain.AsyncVerify")
	defer span.End()

	if block.sigJob != nil {
		return nil
	}
	sigJob, err := p.authVerificationWorkers.NewJob(len(block.StatelessBlock.Txs))
	if err != nil {
		return err
	}
	block.sigJob = sigJob

	// Setup signature verification job
	_, sigVerifySpan := p.tracer.Start(ctx, "Chain.AsyncVerify.verifySignatures") //nolint:spancheck

	batchVerifier := NewAuthBatch(p.authVM, sigJob, block.authCounts)
	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	for _, tx := range block.StatelessBlock.Txs {
		unsignedTxBytes, err := tx.UnsignedBytes()
		if err != nil {
			return err //nolint:spancheck
		}
		batchVerifier.Add(unsignedTxBytes, tx.Auth)
	}
	return nil
}
