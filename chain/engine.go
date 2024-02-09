package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/state"
	"go.uber.org/zap"
)

type engineJob struct {
	parentTimestamp int64
	blk             *StatelessBlock
	chunks          chan *Chunk
}

type output struct {
	txs          map[ids.ID]*blockLoc
	view         merkledb.View
	chunkResults [][]*Result

	startRoot  ids.ID
	chunks     []*FilteredChunk
	feeManager *FeeManager
}

// Engine is in charge of orchestrating the execution of
// accepted chunks.
//
// TODO: put in VM?
type Engine struct {
	vm   VM
	done chan struct{}

	backlog chan *engineJob

	outputsLock   sync.RWMutex
	outputs       map[uint64]*output
	largestOutput *uint64
}

func NewEngine(vm VM, maxBacklog int) *Engine {
	// TODO: strategically use old timestamps on blocks when catching up after processing
	// surge to maximize chunk inclusion
	return &Engine{
		vm:   vm,
		done: make(chan struct{}),

		backlog: make(chan *engineJob, maxBacklog),

		outputs: make(map[uint64]*output),
	}
}

func (e *Engine) Run() {
	defer close(e.done)
	log := e.vm.Logger()

	// Get last accepted state
	parentView := state.View(e.vm.ForceState()) // TODO: state may not be ready at this point

	for {
		select {
		case job := <-e.backlog:
			estart := time.Now()
			ctx := context.Background() // TODO: cleanup
			r := e.vm.Rules(job.blk.StatefulBlock.Timestamp)

			// Fetch parent height key and ensure block height is valid
			heightKey := HeightKey(e.vm.StateManager().HeightKey())
			parentHeightRaw, err := parentView.GetValue(ctx, heightKey)
			if err != nil {
				panic(err)
			}
			parentHeight := binary.BigEndian.Uint64(parentHeightRaw)
			if job.blk.Height() != parentHeight+1 {
				// TODO: re-execute previous blocks to get to required state
				panic(ErrInvalidBlockHeight)
			}

			// Compute fees
			feeKey := FeeKey(e.vm.StateManager().FeeKey())
			feeRaw, err := parentView.GetValue(ctx, feeKey)
			if err != nil {
				panic(err)
			}
			parentFeeManager := NewFeeManager(feeRaw)
			feeManager, err := parentFeeManager.ComputeNext(job.parentTimestamp, job.blk.StatefulBlock.Timestamp, r)
			if err != nil {
				panic(err)
			}

			// Process chunks
			//
			// We know that if any new available chunks are added that block context must be non-nil (so warp messages will be processed).
			p := NewProcessor(e.vm, e, job.blk.bctx, len(job.blk.AvailableChunks), job.blk.StatefulBlock.Timestamp, parentView, feeManager, r)
			chunks := make([]*Chunk, 0, len(job.blk.AvailableChunks))
			for chunk := range job.chunks {
				// Handle case where vm is shutting down (only case where chunk could be nil)
				//
				// We will continue trying to fetch chunk until we find it on the network layer.
				if chunk == nil {
					return
				}

				// Handle fetched chunk
				if err := p.Add(ctx, len(chunks), chunk); err != nil {
					panic(err)
				}
				chunks = append(chunks, chunk)
			}
			txSet, ts, chunkResults, err := p.Wait()
			if err != nil {
				panic(err)
			}

			// Create FilteredChunks
			filteredChunks := make([]*FilteredChunk, len(chunkResults))
			for i, chunkResult := range chunkResults {
				var (
					validResults = make([]*Result, 0, len(chunkResult))
					chunk        = chunks[i]
					cert         = job.blk.AvailableChunks[i]
					txs          = make([]*Transaction, 0, len(chunkResult))

					warpResults set.Bits64
					warpCount   uint
				)
				for j, txResult := range chunkResult {
					tx := chunk.Txs[j]
					if !txResult.Valid {
						// Remove txID from txSet if it was invalid and
						// it was the first txID of its kind seen in the block.
						if bl, ok := txSet[tx.ID()]; ok {
							if bl.chunk == i && bl.index == j {
								delete(txSet, tx.ID())
							}
						}

						// TODO: track invalid tx count
						continue
					}
					validResults = append(validResults, txResult)
					txs = append(txs, tx)
					if tx.WarpMessage != nil {
						if txResult.WarpVerified {
							warpResults.Add(warpCount)
						}
						warpCount++
					}
				}
				filteredChunks[i] = &FilteredChunk{
					Chunk:    cert.Chunk,
					Producer: chunk.Producer,

					Txs:         txs,
					WarpResults: warpResults,
				}

				// As soon as execution of transactions is finished, let the VM know so that it
				// can notify subscribers.
				//
				// TODO: allow for querying agains the executed tip of state rather than the accepted one (which
				// will be artificially delayed to give time to fetch missing chunks)
				//
				// TODO: handle restart case where block may be sent twice?
				//
				// TODO: send async?
				e.vm.Executed(ctx, job.blk.Height(), filteredChunks[i], validResults)
			}

			// Update chain metadata
			heightKeyStr := string(heightKey)
			feeKeyStr := string(feeKey)
			keys := make(state.Keys)
			keys.Add(heightKeyStr, state.Write) // TODO: can probably remove this?
			keys.Add(feeKeyStr, state.Write)
			tsv := ts.NewView(keys, map[string][]byte{
				heightKeyStr: parentHeightRaw,
				feeKeyStr:    parentFeeManager.Bytes(),
			})
			if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, job.blk.StatefulBlock.Height)); err != nil {
				panic(err)
			}
			if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
				panic(err)
			}
			tsv.Commit()

			// Get start root
			startRoot, err := parentView.GetMerkleRoot(ctx)
			if err != nil {
				panic(err)
			}

			// Create new view and kickoff generation
			view, err := ts.ExportMerkleDBView(ctx, e.vm.Tracer(), parentView)
			if err != nil {
				panic(err)
			}
			go func() {
				start := time.Now()
				root, err := view.GetMerkleRoot(ctx)
				if err != nil {
					log.Error("merkle root generation failed", zap.Error(err))
					return
				}
				log.Info("merkle root generated",
					zap.Uint64("height", job.blk.StatefulBlock.Height),
					zap.Stringer("blkID", job.blk.ID()),
					zap.Stringer("root", root),
					zap.Duration("t", time.Since(start)),
				)
			}()

			// Store and update parent view
			e.outputsLock.Lock()
			e.outputs[job.blk.StatefulBlock.Height] = &output{
				txs:          txSet,
				view:         view,
				chunkResults: chunkResults,
				startRoot:    startRoot,
				chunks:       filteredChunks,
				feeManager:   feeManager,
			}
			e.largestOutput = &job.blk.StatefulBlock.Height
			e.outputsLock.Unlock()
			parentView = view

			log.Info(
				"executed block",
				zap.Stringer("blkID", job.blk.ID()),
				zap.Uint64("height", job.blk.StatefulBlock.Height),
				zap.Duration("t", time.Since(estart)),
			)

		case <-e.vm.StopChan():
			return
		}
	}
}

func (e *Engine) Execute(blk *StatelessBlock) {
	// Request chunks for processing when ready
	chunks := make(chan *Chunk, len(blk.AvailableChunks))
	go e.vm.RequestChunks(blk.AvailableChunks, chunks)

	// Enqueue job
	e.backlog <- &engineJob{
		parentTimestamp: blk.parent.StatefulBlock.Timestamp,
		blk:             blk,
		chunks:          chunks,
	}
}

func (e *Engine) Results(height uint64) (ids.ID /* StartRoot */, []ids.ID /* Executed Chunks */, error) {
	// TODO: handle case where never started execution (state sync)
	e.outputsLock.RLock()
	defer e.outputsLock.RUnlock()

	if output, ok := e.outputs[height]; ok {
		filteredIDs := make([]ids.ID, len(output.chunks))
		for i, chunk := range output.chunks {
			id, err := chunk.ID()
			if err != nil {
				return ids.Empty, nil, err
			}
			filteredIDs[i] = id
		}
		return output.startRoot, filteredIDs, nil
	}
	return ids.Empty, nil, fmt.Errorf("%w: results not found for %d", errors.New("not found"), height)
}

// TODO: cleanup this function signautre
func (e *Engine) Commit(ctx context.Context, height uint64) (*FeeManager, [][]*Result, []*FilteredChunk, error) {
	// TODO: fetch results prior to commit to reduce observed finality (state won't be queryable yet but can send block/results)
	e.outputsLock.Lock()
	output, ok := e.outputs[height]
	if !ok {
		e.outputsLock.Unlock()
		return nil, nil, nil, fmt.Errorf("%w: %d", errors.New("not outputs found at height"), height)
	}
	delete(e.outputs, height)
	if e.largestOutput != nil && *e.largestOutput == height {
		e.largestOutput = nil
	}
	e.outputsLock.Unlock()
	return output.feeManager, output.chunkResults, output.chunks, output.view.CommitToDB(ctx)
}

func (e *Engine) IsRepeatTx(
	ctx context.Context,
	txs []*Transaction,
	marker set.Bits,
) (set.Bits, error) {
	e.outputsLock.RLock()
	if e.largestOutput != nil {
		for start := *e.largestOutput; start > 0; start-- {
			output, ok := e.outputs[start]
			if !ok {
				break
			}
			for i, tx := range txs {
				if marker.Contains(i) {
					continue
				}
				if _, ok := output.txs[tx.ID()]; ok {
					marker.Add(i)
				}
			}
		}
	}
	e.outputsLock.RUnlock()
	return e.vm.IsRepeatTx(ctx, txs, marker), nil
}

func (e *Engine) Done() {
	<-e.done
}
