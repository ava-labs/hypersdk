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
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/fetcher"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	oteltrace "go.opentelemetry.io/otel/trace"
)

type ExecutionBlock struct {
	*StatelessBlock

	// authCounts can be used by batch signature verification
	// to preallocate memory
	authCounts map[uint8]int
	txsSet     set.Set[ids.ID]
}

type OutputBlock struct {
	*ExecutionBlock

	View             merkledb.View
	ExecutionResults *ExecutionResults
}

type blockContext struct {
	height     uint64
	timestamp  int64
	feeManager *fees.Manager
}

func NewExecutionBlock(block *StatelessBlock) *ExecutionBlock {
	authCounts := make(map[uint8]int)
	txsSet := set.NewSet[ids.ID](len(block.Txs))
	for _, tx := range block.Txs {
		txsSet.Add(tx.GetID())
		authCounts[tx.Auth.GetTypeID()]++
	}

	return &ExecutionBlock{
		authCounts:     authCounts,
		txsSet:         txsSet,
		StatelessBlock: block,
	}
}

func (b *ExecutionBlock) Contains(id ids.ID) bool {
	return b.txsSet.Contains(id)
}

func (b *ExecutionBlock) GetContainers() []*Transaction {
	return b.StatelessBlock.Txs
}

type Processor struct {
	tracer                  trace.Tracer
	log                     logging.Logger
	ruleFactory             RuleFactory
	authVerificationWorkers workers.Workers
	authEngines             AuthEngines
	metadataManager         MetadataManager
	balanceHandler          BalanceHandler
	validityWindow          ValidityWindow
	metrics                 *ChainMetrics
	config                  Config
}

func NewProcessor(
	tracer trace.Tracer,
	log logging.Logger,
	ruleFactory RuleFactory,
	authVerificationWorkers workers.Workers,
	authEngines AuthEngines,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
	validityWindow ValidityWindow,
	metrics *ChainMetrics,
	config Config,
) *Processor {
	return &Processor{
		tracer:                  tracer,
		log:                     log,
		ruleFactory:             ruleFactory,
		authVerificationWorkers: authVerificationWorkers,
		authEngines:             authEngines,
		metadataManager:         metadataManager,
		balanceHandler:          balanceHandler,
		validityWindow:          validityWindow,
		metrics:                 metrics,
		config:                  config,
	}
}

func (p *Processor) Execute(
	ctx context.Context,
	parentView merkledb.View,
	b *ExecutionBlock,
	isNormalOp bool,
) (*OutputBlock, error) {
	ctx, span := p.tracer.Start(ctx, "Chain.Execute")
	defer span.End()

	var (
		r   = p.ruleFactory.GetRules(b.Tmstmp)
		log = p.log
	)

	// Perform basic correctness checks before doing any expensive work
	if b.Tmstmp > time.Now().Add(FutureBound).UnixMilli() {
		return nil, ErrTimestampTooLate
	}
	// create and start signature verification job async
	sigJob, err := p.verifySignatures(ctx, b)
	if err != nil {
		return nil, err
	}

	blockContext, err := p.createBlockContext(ctx, parentView, b, r)
	if err != nil {
		return nil, err
	}

	if isNormalOp {
		if err := p.validityWindow.VerifyExpiryReplayProtection(ctx, b); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDuplicateTx, err)
		}
	}

	// Process transactions
	results, ts, err := p.executeTxs(ctx, b, parentView, blockContext.feeManager, r)
	if err != nil {
		return nil, fmt.Errorf("failed to execute txs: %w", err)
	}

	tsv := ts.NewView(
		state.CompletePermissions,
		state.ImmutableStorage(map[string][]byte{}),
		0,
	)
	if err := p.writeBlockContext(
		ctx,
		tsv,
		blockContext,
	); err != nil {
		return nil, err
	}
	tsv.Commit()

	// Verify parent root
	//
	// Because fee bytes are recorded in state, it is sufficient to check the state root
	// to verify all fee calculations were correct.
	if err := p.verifyParentRoot(ctx, parentView, b.StateRoot); err != nil {
		return nil, err
	}

	// Ensure signatures are verified
	if err := p.waitSignatures(ctx, sigJob); err != nil {
		return nil, err
	}

	// Get view from [tstate] after processing all state transitions
	p.metrics.stateChanges.Add(float64(ts.PendingChanges()))
	p.metrics.stateOperations.Add(float64(ts.OpIndex()))

	view, err := createView(ctx, p.tracer, parentView, ts.ChangedKeys())
	if err != nil {
		return nil, err
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

	return &OutputBlock{
		ExecutionBlock: b,
		View:           view,
		ExecutionResults: &ExecutionResults{
			Results:       results,
			UnitPrices:    blockContext.feeManager.UnitPrices(),
			UnitsConsumed: blockContext.feeManager.UnitsConsumed(),
		},
	}, nil
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
	ctx, span := p.tracer.Start(ctx, "Chain.Execute.executeTxs")
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
				stateKeys,
				state.ImmutableStorage(storage),
				len(stateKeys),
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

	p.metrics.txsVerified.Add(float64(numTxs))

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}

// verifySignatures creates and kicks off signature verification job for the provided block
// Assumes that the executionBlock's authCounts field has been populated correctly during construction
func (p *Processor) verifySignatures(ctx context.Context, block *ExecutionBlock) (workers.Job, error) {
	sigJob, err := p.authVerificationWorkers.NewJob(len(block.StatelessBlock.Txs))
	if err != nil {
		return nil, err
	}

	// Setup signature verification job
	_, sigVerifySpan := p.tracer.Start(ctx, "Chain.Execute.verifySignatures")

	batchVerifier := NewAuthBatch(p.log, p.authEngines, sigJob, block.authCounts)
	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	for _, tx := range block.StatelessBlock.Txs {
		unsignedTxBytes := tx.UnsignedBytes()
		batchVerifier.Add(unsignedTxBytes, tx.Auth)
	}
	return sigJob, nil
}

func (p *Processor) waitSignatures(ctx context.Context, sigJob workers.Job) error {
	_, span := p.tracer.Start(ctx, "Chain.Execute.waitSignatures")
	defer span.End()

	start := time.Now()
	err := sigJob.Wait()
	if err != nil {
		return fmt.Errorf("signatures failed verification: %w", err)
	}
	p.metrics.waitSignaturesCount.Inc()
	p.metrics.waitSignaturesSum.Add(float64(time.Since(start)))
	return nil
}

// createBlockContext extracts and verifies the block context from the parent view
// and provided block
func (p *Processor) createBlockContext(
	ctx context.Context,
	im state.Immutable,
	block *ExecutionBlock,
	r Rules,
) (blockContext, error) {
	_, span := p.tracer.Start(ctx, "Chain.Execute.createBlockContext")
	defer span.End()

	// Get parent height
	heightKey := HeightKey(p.metadataManager.HeightPrefix())
	parentHeightRaw, err := im.GetValue(ctx, heightKey)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToFetchParentHeight, err)
	}
	parentHeight, err := database.ParseUInt64(parentHeightRaw)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToParseParentHeight, err)
	}
	if block.Hght != parentHeight+1 {
		return blockContext{}, fmt.Errorf("%w: block height %d != parentHeight (%d) + 1", ErrInvalidBlockHeight, block.Hght, parentHeight)
	}

	// Get parent timestamp
	timestampKey := TimestampKey(p.metadataManager.TimestampPrefix())
	parentTimestampRaw, err := im.GetValue(ctx, timestampKey)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToFetchParentTimestamp, err)
	}
	parsedParentTimestamp, err := database.ParseUInt64(parentTimestampRaw)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToParseParentTimestamp, err)
	}

	// Confirm block timestamp is valid
	//
	// Parent may not be available (if we preformed state sync), so we
	// can't rely on being able to fetch it during verification.
	parentTimestamp := int64(parsedParentTimestamp)
	if minBlockGap := r.GetMinBlockGap(); block.Tmstmp < parentTimestamp+minBlockGap {
		return blockContext{}, fmt.Errorf("%w: block timestamp %d < parentTimestamp (%d) + minBlockGap (%d)", ErrTimestampTooEarly, block.Tmstmp, parentTimestamp, minBlockGap)
	}
	if len(block.StatelessBlock.Txs) == 0 && block.Tmstmp < parentTimestamp+r.GetMinEmptyBlockGap() {
		return blockContext{}, fmt.Errorf("%w: timestamp (%d) < parentTimestamp (%d) + minEmptyBlockGap (%d)", ErrTimestampTooEarlyEmptyBlock, block.Tmstmp, parentTimestamp, r.GetMinEmptyBlockGap())
	}

	// Calculate fee manager for this block
	feeKey := FeeKey(p.metadataManager.FeePrefix())
	parentFeeRaw, err := im.GetValue(ctx, feeKey)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToFetchParentFee, err)
	}
	parentFeeManager := fees.NewManager(parentFeeRaw)
	blockFeeManager := parentFeeManager.ComputeNext(block.Tmstmp, r)

	return blockContext{
		height:     block.Hght,
		timestamp:  block.Tmstmp,
		feeManager: blockFeeManager,
	}, nil
}

func (p *Processor) writeBlockContext(
	ctx context.Context,
	mu state.Mutable,
	blockCtx blockContext,
) error {
	var (
		heightKey    = HeightKey(p.metadataManager.HeightPrefix())
		timestampKey = TimestampKey(p.metadataManager.TimestampPrefix())
		feeKey       = FeeKey(p.metadataManager.FeePrefix())
	)
	if err := mu.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, blockCtx.height)); err != nil {
		return fmt.Errorf("failed to insert height into state: %w", err)
	}
	if err := mu.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(blockCtx.timestamp))); err != nil {
		return fmt.Errorf("failed to insert timestamp into state: %w", err)
	}
	if err := mu.Insert(ctx, feeKey, blockCtx.feeManager.Bytes()); err != nil {
		return fmt.Errorf("failed to insert fee manager into state: %w", err)
	}
	return nil
}

func (p *Processor) verifyParentRoot(
	ctx context.Context,
	parentView merkledb.View,
	stateRoot ids.ID,
) error {
	_, span := p.tracer.Start(ctx, "Chain.Execute.verifyParentRoot")
	defer span.End()

	start := time.Now()
	computedRoot, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to calculate parent state root: %w", err)
	}
	p.metrics.waitRootCount.Inc()
	p.metrics.waitRootSum.Add(float64(time.Since(start)))
	if stateRoot != computedRoot {
		return fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			stateRoot,
		)
	}
	return nil
}

func createView(ctx context.Context, tracer trace.Tracer, parentView state.View, stateDiff map[string]maybe.Maybe[[]byte]) (merkledb.View, error) {
	ctx, span := tracer.Start(
		ctx, "Chain.CreateView",
		oteltrace.WithAttributes(
			attribute.Int("items", len(stateDiff)),
		),
	)
	defer span.End()

	return parentView.NewView(ctx, merkledb.ViewChanges{
		MapOps:       stateDiff,
		ConsumeBytes: true,
	})
}
