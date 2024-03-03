package chain

import (
	"context"
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/sourcegraph/conc/stream"
	"go.uber.org/zap"
)

const numTxs = 50000 // TODO: somehow estimate this (needed to ensure no backlog)

type Processor struct {
	vm  VM
	eng *Engine

	authStream *stream.Stream

	latestPHeight *uint64
	epochInfo     []*EpochInfo

	timestamp int64
	epoch     uint64
	im        state.Immutable
	r         Rules
	sm        StateManager
	cacheLock sync.RWMutex
	cache     map[string]*fetchData
	exectutor *executor.Executor
	ts        *tstate.TState

	txs     map[ids.ID]*blockLoc
	results [][]*Result

	claimL         sync.Mutex
	frozenSponsors set.Set[string]
	claims         []*Transaction
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
	latestPHeight *uint64, epochInfo []*EpochInfo,
	chunks int, timestamp int64, im state.Immutable, r Rules,
) *Processor {
	stream := stream.New()
	stream.WithMaxGoroutines(10) // TODO: use config
	return &Processor{
		vm:  vm,
		eng: eng,

		authStream: stream,

		latestPHeight: latestPHeight,
		epochInfo:     epochInfo,

		timestamp: timestamp,
		epoch:     utils.Epoch(timestamp, r.GetEpochDuration()),
		im:        im,
		r:         r,
		sm:        vm.StateManager(),
		cache:     make(map[string]*fetchData, numTxs),
		// Executor is shared across all chunks, this means we don't need to "wait" at the end of each chunk to continue
		// processing transactions.
		exectutor: executor.New(numTxs, vm.GetTransactionExecutionCores(), vm.GetExecutorVerifyRecorder()),
		ts:        tstate.New(numTxs * 2),

		txs:     make(map[ids.ID]*blockLoc, numTxs),
		results: make([][]*Result, chunks),

		frozenSponsors: set.NewSet[string](4),
		claims:         []*Transaction{},
	}
}

func (p *Processor) process(ctx context.Context, chunkIndex int, txIndex int, pchainHeight uint64, units Dimensions, tx *Transaction) {
	stateKeys, err := tx.StateKeys(p.sm)
	if err != nil {
		p.vm.Logger().Warn("could not compute state keys", zap.Stringer("txID", tx.ID()), zap.Error(err))
		p.results[chunkIndex][txIndex] = &Result{Valid: false}
		return
	}
	p.exectutor.Run(stateKeys, func() error {
		// Fetch keys from cache
		var (
			reads    = make(map[string]uint16, len(stateKeys))
			storage  = make(map[string][]byte, len(stateKeys))
			toLookup = make([]string, 0, len(stateKeys))
		)
		p.cacheLock.RLock()
		for k := range stateKeys {
			if v, ok := p.cache[k]; ok {
				reads[k] = v.chunks
				if v.exists {
					storage[k] = v.v
				}
				continue
			}
			toLookup = append(toLookup, k)
		}
		p.cacheLock.RUnlock()

		// Fetch keys from disk
		var toCache map[string]*fetchData
		if len(toLookup) > 0 {
			toCache = make(map[string]*fetchData, len(toLookup))
			for _, k := range toLookup {
				v, err := p.im.GetValue(ctx, []byte(k))
				if errors.Is(err, database.ErrNotFound) {
					reads[k] = 0
					toCache[k] = &fetchData{nil, false, 0}
					continue
				} else if err != nil {
					return err
				}
				// We verify that the [NumChunks] is already less than the number
				// added on the write path, so we don't need to do so again here.
				numChunks, ok := keys.NumChunks(v)
				if !ok {
					return ErrInvalidKeyValue
				}
				reads[k] = numChunks
				toCache[k] = &fetchData{v, true, numChunks}
				storage[k] = v
			}
		}

		// Execute transaction
		//
		// It is critical we explicitly set the scope before each transaction is
		// processed
		tsv := p.ts.NewView(stateKeys, storage)

		// Ensure we have enough funds to pay fees
		fee, err := MulSum(p.r.GetUnitPrices(), units)
		if err != nil {
			// This is an unexpected error
			return err
		}
		if tx.Base.MaxFee < fee {
			// This should be checked by chunk producer before inclusion.
			p.vm.Logger().Warn("fee is greater than max fee", zap.Stringer("txID", tx.ID()), zap.Uint64("max", tx.Base.MaxFee), zap.Uint64("fee", fee))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			return nil
		}

		// Deduct fees
		sponsor := tx.Auth.Sponsor()
		ok, err := p.sm.CanDeduct(ctx, sponsor, tsv, fee)
		if err != nil {
			return err
		}
		if !ok {
			p.vm.Logger().Warn("insufficient funds", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}

			// Enqueue for claim filing
			p.claimL.Lock()
			p.claims = append(p.claims, tx)
			p.frozenSponsors.Add(string(sponsor[:]))
			p.claimL.Unlock()
			return nil
		}

		// Wait to perform warp verification until we know the transaction can pay fees
		warpVerified := p.verifyWarpMessage(ctx, pchainHeight, tx)

		// Execute transaction
		//
		// Also deducts fees
		result, err := tx.Execute(ctx, reads, p.sm, p.r, tsv, p.timestamp, warpVerified)
		if err != nil {
			p.vm.Logger().Warn("execution failure", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			return nil
		}
		result.Valid = true
		result.WarpVerified = warpVerified
		p.results[chunkIndex][txIndex] = result

		// Commit results to parent [TState]
		tsv.Commit()

		// TODO: pay fees to validator that included (based on % of signatures)
		// Should wait for end to avoid conflicts

		// Update key cache
		if len(toCache) > 0 {
			p.cacheLock.Lock()
			for k := range toCache {
				p.cache[k] = toCache[k]
			}
			p.cacheLock.Unlock()
		}
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

	// Confirm that chunk is well-formed
	//
	// All of these can be avoided by chunk producer.
	cid, err := chunk.ID() // TODO: make this panic on err
	if err != nil {
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
	repeats, err := p.eng.IsRepeatTx(ctx, chunk.Txs, set.NewBits())
	if err != nil {
		p.vm.Logger().Warn("chunk has repeat transaction", zap.Stringer("chunk", cid), zap.Error(err))
		p.markChunkTxsInvalid(chunkIndex, chunkTxs)
		return
	}
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

	// Process chunk transactions
	for ri, rtx := range chunk.Txs {
		// TODO: remove in go1.22
		txIndex := ri
		tx := rtx

		// Perform syntactic verification
		//
		// We don't care whether this transaction is in the current epoch or the next.
		units, err := tx.SyntacticVerify(ctx, p.sm, p.r, p.timestamp)
		if err != nil {
			p.vm.Logger().Warn("transaction is invalid", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}
		if tx.Base.Timestamp > chunk.Slot {
			p.vm.Logger().Warn("base transaction has timestamp after slot", zap.Stringer("txID", tx.ID()))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}

		// Check that transaction isn't a duplicate
		_, seen := p.txs[tx.ID()]
		if repeats.Contains(txIndex) || seen {
			p.vm.Logger().Warn("transaction is a duplicate", zap.Stringer("txID", tx.ID()))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}

		// Check that height is set for epoch
		txEpoch := utils.Epoch(tx.Base.Timestamp, p.r.GetEpochDuration())
		epochInfo := p.epochInfo[txEpoch-p.epoch]
		if epochInfo == nil {
			// We can't verify tx partition if this is the case
			p.vm.Logger().Warn("pchainHeight not set for epoch", zap.Stringer("txID", tx.ID()))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}
		// If this passes, we know that latest pHeight must be non-nil

		// Check that transaction is in right partition
		parition, err := p.vm.AddressPartition(ctx, epochInfo.PHeight, tx.Auth.Sponsor())
		if err != nil {
			p.vm.Logger().Warn("unable to compute tx partition", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkIndex][txIndex] = &Result{Valid: false}
			continue
		}
		if parition != chunk.Producer {
			p.vm.Logger().Warn("tx in wrong partition", zap.Stringer("txID", tx.ID()))
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

		// Check that transaction isn't frozen (can avoid state lookups)
		//
		// Need to wait to enqueue until after verify signature.
		//
		// TODO: We still want to claim if there is an available bond for our epoch even if frozen in another.
		var frozen bool
		sponsor := tx.Auth.Sponsor()
		ok, err := p.sm.IsFrozen(ctx, sponsor, txEpoch, p.im)
		if err != nil {
			panic(err)
		}
		if !ok {
			frozen = true
		} else {
			// Avoid unnecessary state lookups if we already marked as frozen
			p.claimL.Lock()
			if p.frozenSponsors.Contains(string(sponsor[:])) {
				frozen = true
			}
			p.claimL.Unlock()
		}

		// Enqueue transaction for execution
		p.authStream.Go(func() stream.Callback {
			msg, err := tx.Digest()
			if err != nil {
				p.vm.Logger().Warn("could not compute tx digest", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}
				return func() {}
			}
			if p.vm.GetVerifyAuth() {
				if err := tx.Auth.Verify(ctx, msg); err != nil {
					p.vm.Logger().Warn("auth verification failed", zap.Stringer("txID", tx.ID()), zap.Error(err))
					p.results[chunkIndex][txIndex] = &Result{Valid: false}
					return func() {}
				}
			}
			if frozen {
				p.vm.Logger().Warn("dropping tx from frozen sponsor", zap.Stringer("txID", tx.ID()))
				p.results[chunkIndex][txIndex] = &Result{Valid: false}

				// Enqueue for claim filing
				p.claimL.Lock()
				p.claims = append(p.claims, tx)
				p.frozenSponsors.Add(string(sponsor[:]))
				p.claimL.Unlock()
				return func() {}
			}
			return func() { p.process(ctx, chunkIndex, txIndex, *p.latestPHeight, units, tx) }
		})
	}
}

func (p *Processor) Wait() (map[ids.ID]*blockLoc, *tstate.TState, [][]*Result, []*Transaction, error) {
	p.authStream.Wait()
	return p.txs, p.ts, p.results, p.claims, p.exectutor.Wait()
}
