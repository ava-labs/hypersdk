package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"go.uber.org/zap"
)

const numTxs = 50000 // TODO: somehow estimate this (needed to ensure no backlog)

var ErrNotReady = errors.New("not ready")

type ChunkResult struct {
	// Warp      set.Bits
	Execution []*Result
}

type Processor struct {
	chunks chan *Chunk

	vm VM

	l        sync.Mutex
	complete bool
	err      error

	timestamp    int64
	im           state.Immutable
	feeManager   *FeeManager
	r            Rules
	sm           StateManager
	cacheLock    sync.RWMutex
	cache        map[string]*fetchData
	exectutor    *executor.Executor
	ts           *tstate.TState
	warpMessages map[ids.ID]*warpJob

	results []*ChunkResult
}

func NewProcessor(
	vm VM,
	chunks int,
) *Processor {
	return &Processor{
		vm: vm,

		chunks:  make(chan *Chunk, chunks),
		results: make([]*ChunkResult, chunks),
	}
}

// warpJob is used to signal to a listner that a *warp.Message has been
// verified.
type warpJob struct {
	msg          *warp.Message
	signers      int
	verifiedChan chan bool
	verified     bool
	warpNum      int
}

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

// TODO: handle mapping chunk to new chunk
// TODO: new chunk could have warp results + results?
// TODO: kickoff signature verification before begin execution
func (p *Processor) process(ctx context.Context, chunkOrder int, c *Chunk) {
	p.results[chunkOrder] = &ChunkResult{Execution: make([]*Result, len(c.Txs))}
	for ri, rtx := range c.Txs {
		i := ri
		tx := rtx

		stateKeys, err := tx.StateKeys(p.sm)
		if err != nil {
			p.vm.Logger().Warn("could not compute state keys", zap.Stringer("txID", tx.ID()), zap.Error(err))
			p.results[chunkOrder].Execution[i] = &Result{Valid: false}
			continue
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
			if err := tx.PreExecute(ctx, p.feeManager, p.sm, p.r, tsv, p.timestamp); err != nil {
				p.vm.Logger().Warn("pre-execution failure", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkOrder].Execution[i] = &Result{Valid: false}
				return nil
			}

			// Wait to execute transaction until we have the warp result processed.
			var warpVerified bool
			warpMsg, ok := p.warpMessages[tx.ID()]
			if ok {
				select {
				case warpVerified = <-warpMsg.verifiedChan:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			result, err := tx.Execute(ctx, p.feeManager, reads, p.sm, p.r, tsv, p.timestamp, ok && warpVerified)
			if err != nil {
				p.vm.Logger().Warn("execution failure", zap.Stringer("txID", tx.ID()), zap.Error(err))
				p.results[chunkOrder].Execution[i] = &Result{Valid: false}
				return nil
			}
			p.results[chunkOrder].Execution[i] = result

			// Update block metadata with units actually consumed (if more is consumed than block allows, we will non-deterministically
			// exit with an error based on which tx over the limit is processed first)
			//
			// TODO: we won't know this when just including certs?
			if ok, d := p.feeManager.Consume(result.Consumed, p.r.GetMaxBlockUnits()); !ok {
				return fmt.Errorf("%w: %d too large", ErrInvalidUnitsConsumed, d)
			}

			// Commit results to parent [TState]
			tsv.Commit()

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
}

func (p *Processor) Run(ctx context.Context, timestamp int64, im state.Immutable, feeManager *FeeManager, r Rules) {
	ctx, span := p.vm.Tracer().Start(ctx, "Processor.Run")
	defer span.End()

	// Setup the processor
	p.timestamp = timestamp
	p.im = im
	p.feeManager = feeManager
	p.r = r
	p.sm = p.vm.StateManager()
	p.cache = make(map[string]*fetchData, numTxs)
	// Executor is shared across all chunks, this means we don't need to "wait" at the end of each chunk to continue
	// processing transactions.
	p.exectutor = executor.New(numTxs, p.vm.GetTransactionExecutionCores(), p.vm.GetExecutorVerifyRecorder())
	p.ts = tstate.New(numTxs * 2)
	p.warpMessages = map[ids.ID]*warpJob{}

	// Handle chunks
	var chunkOrder int
	for {
		select {
		case c, ok := <-p.chunks:
			// Handle graceful exit
			if !ok {
				err := p.exectutor.Wait()
				p.l.Lock()
				p.complete = true // only mark complete once chunks have been dequeued
				if p.err == nil {
					p.err = err
				}
				p.l.Unlock()
				return
			}

			// Attempt to exit early
			p.l.Lock()
			if p.err != nil {
				p.l.Unlock()
				continue
			}

			// Process chunk
			p.process(ctx, chunkOrder, c)
			chunkOrder++

		case <-ctx.Done():
			p.l.Lock()
			if p.err != nil {
				p.err = ctx.Err()
			}
			p.l.Unlock()
			return
		}
	}
}

// Allows processing to start before all chunks are acquired.
func (p *Processor) Add(chunk *Chunk) {
	p.chunks <- chunk
}

func (p *Processor) Done() {
	close(p.chunks)
}

func (p *Processor) Results() ([]*ChunkResult, error) {
	p.l.Lock()
	defer p.l.Unlock()

	if !p.complete {
		return nil, ErrNotReady
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.results, p.err
}
