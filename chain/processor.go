package chain

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/workers"
	"go.uber.org/zap"
)

const numTxs = 50000 // TODO: somehow estimate this (needed to ensure no backlog)

type Processor struct {
	vm  VM
	eng *Engine

	latestPHeight *uint64
	epochHeights  []*uint64

	timestamp int64
	epoch     uint64
	im        state.Immutable
	r         Rules
	sm        StateManager
	exectutor *executor.Executor
	ts        *tstate.TState

	txs     map[ids.ID]*blockLoc
	results [][]*Result

	frozenSponsors set.Set[string]

	repeatWait time.Duration

	authWorkers workers.Workers
	authWait    time.Duration

	serialChecks          time.Duration
	chunkUnits            time.Duration
	addProcess            time.Duration
	frozenChecks          time.Duration
	syntacticVerification time.Duration
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
		// Executor is shared across all chunks, this means we don't need to "wait" at the end of each chunk to continue
		// processing transactions.
		exectutor: executor.New(numTxs, vm.GetActionExecutionCores(), vm.GetExecutorRecorder()),
		ts:        tstate.New(numTxs * 2),

		txs:     make(map[ids.ID]*blockLoc, numTxs),
		results: make([][]*Result, chunks),

		frozenSponsors: set.NewSet[string](4),

		authWorkers: workers.NewParallel(vm.GetAuthExecutionCores(), 4), // should never have more than 1 here
	}
}

func (p *Processor) process(ctx context.Context, chunkIndex int, txIndex int, pchainHeight uint64, fee uint64, tx *Transaction) {
	stateKeys, err := tx.StateKeys(p.sm)
	if err != nil {
		p.vm.Logger().Warn("could not compute state keys", zap.Stringer("txID", tx.ID()), zap.Error(err))
		p.results[chunkIndex][txIndex] = &Result{Valid: false}
		return
	}

	p.exectutor.Run(stateKeys, func() error {
		// Execute transaction
		//
		// It is critical we explicitly set the scope before each transaction is
		// processed
		tsv := p.ts.NewView(p.im, stateKeys)

		// Deduct fees
		sponsor := tx.Auth.Sponsor()
		ssponsor := string(sponsor[:])
		ok, err := p.sm.CanDeduct(ctx, sponsor, tsv, fee)
		if err != nil {
			return err
		}
		if !ok || p.frozenSponsors.Contains(ssponsor) {
			p.vm.Logger().Warn("insufficient funds", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false, Freezable: true, Fee: fee}
			p.frozenSponsors.Add(ssponsor)
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
func (p *Processor) Add(ctx context.Context, chunkIndex int, chunk *Chunk, cw *simpleChunkWrapper) {
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
	cid, err := chunk.ID() // TODO: make this panic on err
	if err != nil {
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	if cw != nil {
		if !cw.success {
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
	}

	// Verify chunk signatures
	//
	// We need to do this before we check basic chunk correctness to support
	// optimistic chunk signature verification.
	if p.vm.GetVerifyAuth() && p.vm.NodeID() != chunk.Producer && cw == nil { // trust ourselves
		p.vm.RecordNotVerifiedChunk()
		authJob, err := p.authWorkers.NewJob(len(chunk.Txs))
		if err != nil {
			panic(err)
		}
		batchVerifier := NewAuthBatch(p.vm, authJob, chunk.authCounts)

		for _, tx := range chunk.Txs {
			// Enqueue transaction for execution
			msg, err := tx.Digest()
			if err != nil {
				p.vm.Logger().Warn("could not compute tx digest", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.markChunkTxsInvalid(chunkIndex, chunkTxs)
				return
			}
			if p.vm.IsRPCAuthorized(tx.ID()) {
				p.vm.RecordTxRPCAuthorized()
				continue
			}
			// We can only pre-check transactions that would invalidate the chunk prior to verifying signatures.
			batchVerifier.Add(msg, tx.Auth)
		}
		authStart := time.Now()
		batchVerifier.Done(func() { p.authWait += time.Since(authStart) })
		if err := authJob.Wait(); err != nil {
			p.vm.Logger().Warn("auth verification failed", zap.Stringer("chunkID", cid), zap.Error(err))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
	}

	// Confirm that chunk is well-formed
	//
	//
	// All of these can be avoided by chunk producer.
	serialStart := time.Now()
	repeatStart := time.Now()
	repeats, err := p.eng.IsRepeatTx(ctx, chunk.Txs, set.NewBits())
	if err != nil {
		p.vm.Logger().Warn("chunk has repeat transaction", zap.Stringer("chunk", cid), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	p.repeatWait += time.Since(repeatStart)
	unitsStart := time.Now()
	chunkUnits, err := chunk.Units(p.sm, p.r)
	if err != nil {
		p.vm.Logger().Warn("could not compute chunk units", zap.Stringer("chunk", cid), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	if chunkUnits.Greater(p.r.GetMaxChunkUnits()) {
		p.vm.Logger().Warn("chunk uses more than max units", zap.Stringer("chunk", cid), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	p.chunkUnits += time.Since(unitsStart)

	for txIndex, tx := range chunk.Txs {
		// Perform syntactic verification
		//
		// We don't care whether this transaction is in the current epoch or the next.
		syntacticStart := time.Now()
		units, err := tx.SyntacticVerify(ctx, p.sm, p.r, p.timestamp)
		if err != nil {
			p.vm.Logger().Debug("transaction is invalid", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
		if tx.Base.Timestamp > chunk.Slot {
			p.vm.Logger().Debug("base transaction has timestamp after slot", zap.Stringer("txID", tx.ID()))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
		fee, err := MulSum(p.r.GetUnitPrices(), units)
		if err != nil {
			p.vm.Logger().Debug("cannot compute fees", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
		if tx.Base.MaxFee < fee {
			// This should be checked by chunk producer before inclusion.
			//
			// This can change, however (based on rules/dynamics), so we don't mark all txs as invalid.
			p.vm.Logger().Debug("fee is greater than max fee", zap.Stringer("txID", tx.ID()), zap.Uint64("max", tx.Base.MaxFee), zap.Uint64("fee", fee))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}
		p.syntacticVerification += time.Since(syntacticStart)

		// Check that transaction isn't a duplicate
		_, seen := p.txs[tx.ID()]
		if repeats.Contains(txIndex) || seen {
			p.vm.Logger().Warn("transaction is a duplicate", zap.Stringer("txID", tx.ID()))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}

		// Check that height is set for epoch
		txEpoch := utils.Epoch(tx.Base.Timestamp, p.r.GetEpochDuration())
		epochHeight := p.epochHeights[txEpoch-p.epoch]
		if epochHeight == nil {
			// We can't verify tx partition if this is the case
			p.vm.Logger().Warn("pchainHeight not set for epoch", zap.Stringer("txID", tx.ID()))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
		// If this passes, we know that latest pHeight must be non-nil

		// Check that transaction is in right partition
		parition, err := p.vm.AddressPartition(ctx, *epochHeight, tx.Auth.Sponsor())
		if err != nil {
			p.vm.Logger().Warn("unable to compute tx partition", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}
		if parition != chunk.Producer {
			p.vm.Logger().Warn("tx in wrong partition", zap.Stringer("txID", tx.ID()))
			p.markChunkTxsInvalid(chunkIndex, chunkTxs)
			return
		}

		// If this is the first instance of a transaction in this block,
		// record it in the set.
		//
		// Remove any invalid transactions from this set when all chunks are done processing (otherwise
		// from ever being executed).
		p.txs[tx.ID()] = &blockLoc{chunkIndex, txIndex}

		// Check that transaction isn't frozen (can avoid state lookups)
		//
		// Need to wait to enqueue until after verify signature.
		sponsor := tx.Auth.Sponsor()
		ssponsor := string(sponsor[:])
		// ok, err := p.sm.IsFrozen(ctx, sponsor, txEpoch, p.im)
		// if err != nil {
		// 	panic(err)
		// }
		frozenStart := time.Now()
		frozen := false
		if frozen {
			p.frozenSponsors.Add(ssponsor)
		}
		if p.frozenSponsors.Contains(ssponsor) {
			p.vm.Logger().Warn("dropping tx from frozen sponsor", zap.Stringer("txID", tx.ID()))
			p.results[chunkIndex][txIndex] = &Result{Valid: false, Freezable: true, Fee: fee}
			continue
		}
		p.frozenChecks += time.Since(frozenStart)
		processStart := time.Now()
		p.process(ctx, chunkIndex, txIndex, *p.latestPHeight, fee, tx)
		p.addProcess += time.Since(processStart)
	}
	p.serialChecks += time.Since(serialStart)
}

func (p *Processor) Wait() (map[ids.ID]*blockLoc, *tstate.TState, [][]*Result, error) {
	p.vm.RecordWaitRepeat(p.repeatWait)
	p.authWorkers.Stop()            // must be stopped, otherwise goroutines grow indefinitely
	p.vm.RecordWaitAuth(p.authWait) // we record once so we can see how much of a block was spend waiting (this is not the same as the total time)
	exectutorStart := time.Now()
	if err := p.exectutor.Wait(); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: processor failed", err)
	}
	p.vm.RecordWaitExec(time.Since(exectutorStart))
	p.vm.Logger().Debug(
		"times",
		zap.Duration("serial checks", p.serialChecks),
		zap.Duration("chunk units", p.chunkUnits),
		zap.Duration("add process", p.addProcess),
		zap.Duration("frozen checks", p.frozenChecks),
		zap.Duration("syntactic verification", p.syntacticVerification),
	)
	return p.txs, p.ts, p.results, nil
}

func VerifyChunkSignatures(vm VM, chunk *Chunk) bool {
	authWorkers := workers.NewParallel(vm.GetAuthExecutionCores(), 4) // should never have more than 1 here
	defer authWorkers.Stop()

	authJob, err := authWorkers.NewJob(len(chunk.Txs))
	if err != nil {
		panic(err)
	}
	batchVerifier := NewAuthBatch(vm, authJob, chunk.authCounts)
	for _, tx := range chunk.Txs {
		// Enqueue transaction for execution
		msg, err := tx.Digest()
		if err != nil {
			return false
		}
		if vm.IsRPCAuthorized(tx.ID()) {
			vm.RecordTxRPCAuthorized()
			continue
		}

		// We can only pre-check transactions that would invalidate the chunk prior to verifying signatures.
		batchVerifier.Add(msg, tx.Auth)
	}
	batchVerifier.Done(nil)
	return authJob.Wait() == nil
}
