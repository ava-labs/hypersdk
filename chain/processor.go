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

type ExecutionBlock[T Action[T], A Auth[A]] struct {
	*Block[T, A]

	txsSet set.Set[ids.ID]
}

type OutputBlock[T Action[T], A Auth[A]] struct {
	*ExecutionBlock[T, A]

	View             merkledb.View
	ExecutionResults ExecutionResults
}

type blockContext struct {
	height     uint64
	timestamp  int64
	feeManager *fees.Manager
}

func NewExecutionBlock[T Action[T], A Auth[A]](block *Block[T, A]) *ExecutionBlock[T, A] {
	txsSet := set.NewSet[ids.ID](len(block.Txs))
	for _, tx := range block.Txs {
		txsSet.Add(tx.GetID())
	}

	return &ExecutionBlock[T, A]{
		txsSet: txsSet,
		Block:  block,
	}
}

func (b *ExecutionBlock[T, A]) Contains(id ids.ID) bool {
	return b.txsSet.Contains(id)
}

func (b *ExecutionBlock[T, A]) GetContainers() []*Transaction[T, A] {
	return b.Block.Txs
}

type Processor[T Action[T], A Auth[A]] struct {
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

func NewProcessor[T Action[T], A Auth[A]](
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
) *Processor[T, A] {
	return &Processor[T, A]{
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

func (p *Processor[T, A]) Execute(
	ctx context.Context,
	parentView merkledb.View,
	b *ExecutionBlock[T, A],
	isNormalOp bool,
) (*OutputBlock[T, A], error) {
	ctx, span := p.tracer.Start(ctx, "Chain.Execute")
	defer span.End()

	var (
		r   = p.ruleFactory.GetRules(b.Timestamp)
		log = p.log
	)

	// Perform basic correctness checks before doing any expensive work
	if b.Timestamp > time.Now().Add(FutureBound).UnixMilli() {
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
			zap.Uint64("height", b.Height),
			zap.Stringer("blkID", b.Block.GetID()),
			zap.Stringer("root", root),
		)
		p.metrics.rootCalculatedCount.Inc()
		p.metrics.rootCalculatedSum.Add(float64(time.Since(start)))
	}()

	return &OutputBlock[T, A]{
		ExecutionBlock: b,
		View:           view,
		ExecutionResults: ExecutionResults{
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

func (p *Processor[T, A]) executeTxs(
	ctx context.Context,
	b *ExecutionBlock[T, A],
	im state.Immutable,
	feeManager *fees.Manager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := p.tracer.Start(ctx, "Chain.Execute.executeTxs")
	defer span.End()

	var (
		numTxs = len(b.Block.Txs)
		t      = b.Timestamp

		f       = fetcher.New(im, numTxs, p.config.StateFetchConcurrency)
		e       = executor.New(numTxs, p.config.TransactionExecutionCores, MaxKeyDependencies, p.metrics.executorVerifyRecorder)
		ts      = tstate.New(numTxs * 2) // TODO: tune this heuristic
		results = make([]*Result, numTxs)
	)

	// Fetch required keys and execute transactions
	for li, ltx := range b.Block.Txs {
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
func (p *Processor[T, A]) verifySignatures(ctx context.Context, block *ExecutionBlock[T, A]) (workers.Job, error) {
	sigJob, err := p.authVerificationWorkers.NewJob(len(block.Block.Txs))
	if err != nil {
		return nil, err
	}

	// Setup signature verification job
	_, sigVerifySpan := p.tracer.Start(ctx, "Chain.Execute.verifySignatures") //nolint:spancheck

	batchVerifier := NewAuthBatch(p.authVM, sigJob, block.authCounts)
	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	for _, tx := range block.Block.Txs {
		unsignedTxBytes := tx.UnsignedBytes()
		batchVerifier.Add(unsignedTxBytes, tx.Auth)
	}
	return sigJob, nil
}

func (p *Processor[T, A]) waitSignatures(ctx context.Context, sigJob workers.Job) error {
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
func (p *Processor[T, A]) createBlockContext(
	ctx context.Context,
	im state.Immutable,
	block *ExecutionBlock[T, A],
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
		return blockContext{}, fmt.Errorf("failed to parse parent height from state: %w", err)
	}
	if block.Height != parentHeight+1 {
		return blockContext{}, fmt.Errorf("%w: block height %d != parentHeight (%d) + 1", ErrInvalidBlockHeight, block.Height, parentHeight)
	}

	// Get parent timestamp
	timestampKey := TimestampKey(p.metadataManager.TimestampPrefix())
	parentTimestampRaw, err := im.GetValue(ctx, timestampKey)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToFetchParentHeight, err)
	}
	parsedParentTimestamp, err := database.ParseUInt64(parentTimestampRaw)
	if err != nil {
		return blockContext{}, fmt.Errorf("failed to parse timestamp from state: %w", err)
	}

	// Confirm block timestamp is valid
	//
	// Parent may not be available (if we preformed state sync), so we
	// can't rely on being able to fetch it during verification.
	parentTimestamp := int64(parsedParentTimestamp)
	if minBlockGap := r.GetMinBlockGap(); block.Timestamp < parentTimestamp+minBlockGap {
		return blockContext{}, fmt.Errorf("%w: block timestamp %d < parentTimestamp (%d) + minBlockGap (%d)", ErrTimestampTooEarly, block.Timestamp, parentTimestamp, minBlockGap)
	}
	if len(block.Block.Txs) == 0 && block.Timestamp < parentTimestamp+r.GetMinEmptyBlockGap() {
		return blockContext{}, fmt.Errorf("%w: timestamp (%d) < parentTimestamp (%d) + minEmptyBlockGap (%d)", ErrTimestampTooEarlyEmptyBlock, block.Timestamp, parentTimestamp, r.GetMinEmptyBlockGap())
	}

	// Calculate fee manager for this block
	feeKey := FeeKey(p.metadataManager.FeePrefix())
	parentFeeRaw, err := im.GetValue(ctx, feeKey)
	if err != nil {
		return blockContext{}, fmt.Errorf("%w: %w", ErrFailedToFetchParentFee, err)
	}
	parentFeeManager := fees.NewManager(parentFeeRaw)
	blockFeeManager := parentFeeManager.ComputeNext(block.Timestamp, r)

	return blockContext{
		height:     block.Height,
		timestamp:  block.Timestamp,
		feeManager: blockFeeManager,
	}, nil
}

func (p *Processor[T, A]) writeBlockContext(
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

func (p *Processor[T, A]) verifyParentRoot(
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
