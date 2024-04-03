package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/opool"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

const numTxs = 250_000 // TODO: somehow estimate this (needed to ensure no backlog and enough space)

type Processor struct {
	vm  VM
	eng *Engine

	latestPHeight *uint64
	epochHeights  []*uint64

	timestamp  int64
	epoch      uint64
	im         state.Immutable
	r          Rules
	sm         StateManager
	prechecker *opool.OPool
	executor   *executor.Executor
	ts         *tstate.TState

	txs     map[ids.ID]*blockLoc
	results [][]*Result

	// TODO: track frozen sponsors

	repeatWait time.Duration
	queueWait  time.Duration
	authWait   time.Duration
}

type blockLoc struct {
	chunk int
	index int
}

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

// Only run one processor at once
func NewProcessor(
	vm VM, eng *Engine,
	latestPHeight *uint64, epochHeights []*uint64,
	chunks int, timestamp int64, im state.Immutable, r Rules,
) *Processor {
	return &Processor{
		vm:  vm,
		eng: eng,

		latestPHeight: latestPHeight,
		epochHeights:  epochHeights,

		timestamp: timestamp,
		epoch:     utils.Epoch(timestamp, r.GetEpochDuration()),
		im:        im,
		r:         r,
		sm:        vm.StateManager(),
		// Prechecker and Executor are shared across all chunks, this means we don't need to "wait" at the end of each chunk to continue
		// processing transactions.
		prechecker: opool.New(vm.GetPrecheckCores(), numTxs),
		executor:   executor.New(numTxs, vm.GetActionExecutionCores(), vm.GetExecutorRecorder()),
		ts:         tstate.New(numTxs * 2),

		txs:     make(map[ids.ID]*blockLoc, numTxs),
		results: make([][]*Result, chunks),
	}
}

func (p *Processor) process(ctx context.Context, chunkIndex int, txIndex int, pchainHeight uint64, chunk *Chunk, stateKeys state.Keys, fee uint64, tx *Transaction) {
	p.executor.Run(stateKeys, func() error {
		// Execute transaction
		//
		// It is critical we explicitly set the scope before each transaction is
		// processed
		tsv := p.ts.NewView(p.im, stateKeys)

		// Deduct fees
		sponsor := tx.Auth.Sponsor()
		ok, err := p.sm.CanDeduct(ctx, sponsor, tsv, fee)
		if err != nil {
			return err
		}
		if !ok {
			p.vm.Logger().Warn("insufficient funds", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false, Freezable: true, Fee: fee}
			return nil
		}

		// Wait to perform warp verification until we know the transaction can pay fees
		warpVerified := p.verifyWarpMessage(ctx, pchainHeight, tx)

		// Execute transaction
		//
		// Also deducts fees
		result, err := tx.Execute(ctx, p.sm, p.r, tsv, p.timestamp, warpVerified)
		if err != nil {
			// TODO: this should never happen
			p.vm.Logger().Warn("execution failure", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			return nil
		}
		result.Valid = true
		result.WarpVerified = warpVerified
		p.results[chunkIndex][txIndex] = result

		// Commit results to parent [TState]
		tsv.Commit()
		return nil
	})
}

func (p *Processor) verifyWarpMessage(ctx context.Context, pchainHeight uint64, tx *Transaction) bool {
	if tx.WarpMessage == nil {
		return false
	}

	allowed, num, denom := p.r.GetWarpConfig(tx.WarpMessage.SourceChainID)
	if !allowed {
		p.vm.Logger().
			Warn("unable to verify warp message", zap.Stringer("warpID", tx.WarpMessage.ID()), zap.Error(ErrDisabledChainID))
	}

	// We don't use cached validator set here because we need to fetch
	// external subnet sets.
	if err := tx.WarpMessage.Signature.Verify(
		ctx,
		&tx.WarpMessage.UnsignedMessage,
		p.r.NetworkID(),
		p.vm.ValidatorState(),
		pchainHeight,
		num,
		denom,
	); err != nil {
		p.vm.Logger().
			Warn("unable to verify warp message", zap.Stringer("warpID", tx.WarpMessage.ID()), zap.Error(err))
		return false
	}
	return true
}

func (p *Processor) markChunkTxsInvalid(chunkIndex, count int) {
	for i := 0; i < count; i++ {
		p.results[chunkIndex][i] = &Result{Valid: false}
	}
}

// Allows processing to start before all chunks are acquired.
//
// Chunks MUST be added in order.
//
// Add must not be called concurrently
func (p *Processor) Add(ctx context.Context, chunkIndex int, chunk *Chunk) {
	ctx, span := p.vm.Tracer().Start(ctx, "Processor.Add")
	defer span.End()

	// Kickoff async signature verification (auth + warp)
	//
	// Wait to start any disk lookup until signature verification is done for that transaction.
	//
	// We can't use batch verification because we don't know which transactions
	// may fail auth.
	//
	// Don't wait for all transactions to finish verification to kickoff execution (should
	// be interleaved).
	chunkTxs := len(chunk.Txs)
	p.results[chunkIndex] = make([]*Result, chunkTxs)

	// Verify chunk signatures
	//
	// We need to do this before we check basic chunk correctness to support
	// optimistic chunk signature verification.
	authStart := time.Now()
	authResult := p.vm.GetAuthResult(chunk.ID())
	p.authWait += time.Since(authStart)
	if !authResult {
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}

	// Confirm that chunk is well-formed
	//
	// All of these can be avoided by chunk producer.
	repeatStart := time.Now()
	repeats, err := p.eng.IsRepeatTx(ctx, chunk.Txs, set.NewBits())
	if err != nil {
		p.vm.Logger().Warn("chunk has repeat transaction", zap.Stringer("chunk", chunk.ID()), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	p.repeatWait += time.Since(repeatStart)
	chunkUnits, err := chunk.Units(p.sm, p.r)
	if err != nil {
		p.vm.Logger().Warn("could not compute chunk units", zap.Stringer("chunk", chunk.ID()), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	if !p.r.GetMaxChunkUnits().Greater(chunkUnits) {
		p.vm.Logger().Warn("chunk uses more than max units", zap.Stringer("chunk", chunk.ID()), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}

	// TODO: consider doing some of these checks in parallel
	queueStart := time.Now()
	for rtxIndex, rtx := range chunk.Txs {
		txIndex := rtxIndex
		tx := rtx

		// Check that transaction isn't a duplicate
		_, seen := p.txs[tx.ID()]
		if repeats.Contains(txIndex) || seen {
			p.vm.Logger().Warn("transaction is a duplicate", zap.Stringer("txID", tx.ID()))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}

		// If this is the first instance of a transaction in this block,
		// record it in the set.
		//
		// Remove any invalid transactions from this set when all chunks are done processing (otherwise
		// it would be trivial for a dishonest producer to include a tx in a bad chunk and prevent it
		// from ever being executed).
		p.txs[tx.ID()] = &blockLoc{chunkIndex, txIndex}

		// Perform general checks (that don't require access to state) before adding to executor
		//
		// To maximize performance, we should minimize the time spent in each execution (as it
		// will cause conflicting keys to be blocked).
		p.prechecker.Go(func() (func(), error) {
			// Perform syntactic verification
			//
			// We don't care whether this transaction is in the current epoch or the next.
			units, err := tx.SyntacticVerify(ctx, p.sm, p.r, p.timestamp)
			if err != nil {
				// This is a debug log because it can get very noisy when transactions were included after their timeslot. For a production
				// release, all the logs in this function should probably be DEBUG.
				p.vm.Logger().Debug("transaction is invalid", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}
			if tx.Base.Timestamp > chunk.Slot {
				p.vm.Logger().Warn("base transaction has timestamp after slot", zap.Stringer("txID", tx.ID()))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}
			fee, err := MulSum(p.r.GetUnitPrices(), units)
			if err != nil {
				p.vm.Logger().Warn("cannot compute fees", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}
			if tx.Base.MaxFee < fee {
				// This should be checked by chunk producer before inclusion.
				//
				// This can change, however (based on rules/dynamics), so we don't mark all txs as invalid.
				p.vm.Logger().Warn("fee is greater than max fee", zap.Stringer("txID", tx.ID()), zap.Uint64("max", tx.Base.MaxFee), zap.Uint64("fee", fee))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}

			// Check that height is set for epoch
			txEpoch := utils.Epoch(tx.Base.Timestamp, p.r.GetEpochDuration())
			epochHeight := p.epochHeights[txEpoch-p.epoch]
			if epochHeight == nil {
				// We can't verify tx partition if this is the case
				p.vm.Logger().Warn("pchainHeight not set for epoch", zap.Stringer("txID", tx.ID()))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}
			// If this passes, we know that latest pHeight must be non-nil

			// Check that transaction is in right partition
			parition, err := p.vm.AddressPartition(ctx, txEpoch, *epochHeight, tx.Auth.Sponsor(), tx.Partition())
			if err != nil {
				p.vm.Logger().Warn("unable to compute tx partition", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}
			if parition != chunk.Producer {
				p.vm.Logger().Warn("tx in wrong partition", zap.Stringer("txID", tx.ID()))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}

			// Check that transaction isn't frozen (can avoid state lookups)
			//
			// Need to wait to enqueue until after verify signature.
			stateKeys, err := tx.StateKeys(p.sm)
			if err != nil {
				p.vm.Logger().Warn("could not compute state keys", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return nil, nil
			}
			return func() { p.process(ctx, chunkIndex, txIndex, *p.latestPHeight, chunk, stateKeys, fee, tx) }, nil
		})
	}
	p.queueWait += time.Since(queueStart)
}

func (p *Processor) Wait() (map[ids.ID]*blockLoc, *tstate.TState, [][]*Result, error) {
	p.vm.RecordWaitRepeat(p.repeatWait)
	p.vm.RecordWaitQueue(p.queueWait)
	p.vm.RecordWaitAuth(p.authWait) // we record once so we can see how much of a block was spend waiting (this is not the same as the total time)
	precheckStart := time.Now()
	if err := p.prechecker.Wait(); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: prechecker failed", err)
	}
	p.vm.RecordWaitPrecheck(time.Since(precheckStart))
	exectutorStart := time.Now()
	if err := p.executor.Wait(); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: processor failed", err)
	}
	p.vm.RecordWaitExec(time.Since(exectutorStart))
	return p.txs, p.ts, p.results, nil
}
