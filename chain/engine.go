package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/vilmo"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

type engineJob struct {
	parentTimestamp int64
	blk             *StatelessBlock
	chunks          chan *Chunk
}

type output struct {
	txs          map[ids.ID]*blockLoc
	chunkResults [][]*Result

	chunks   []*FilteredChunk
	checksum ids.ID
}

// Engine is in charge of orchestrating the execution of
// accepted chunks.
//
// TODO: put in VM?
type Engine struct {
	vm   VM
	done chan struct{}

	backlog chan *engineJob

	db *vilmo.Vilmo

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

func (e *Engine) processJob(batch *vilmo.Batch, job *engineJob) error {
	log := e.vm.Logger()
	e.vm.RecordEngineBacklog(-1)

	estart := time.Now()
	ctx := context.Background() // TODO: cleanup

	// Setup state
	r := e.vm.Rules(job.blk.StatefulBlock.Timestamp)

	// Fetch parent height key and ensure block height is valid
	heightKey := HeightKey(e.vm.StateManager().HeightKey())
	parentHeightRaw, err := e.db.Get(ctx, heightKey)
	if err != nil {
		return err
	}
	parentHeight := binary.BigEndian.Uint64(parentHeightRaw)
	if job.blk.Height() != parentHeight+1 {
		// TODO: re-execute previous blocks to get to required state
		return ErrInvalidBlockHeight
	}

	// Fetch latest block context (used for reliable and recent warp verification)
	var (
		pHeight             *uint64
		shouldUpdatePHeight bool
	)
	pHeightKey := PHeightKey(e.vm.StateManager().PHeightKey())
	pHeightRaw, err := e.db.Get(ctx, pHeightKey)
	if err == nil {
		h := binary.BigEndian.Uint64(pHeightRaw)
		pHeight = &h
	}
	if pHeight == nil || *pHeight < job.blk.PHeight { // use latest P-Chain height during verification
		shouldUpdatePHeight = true
		pHeight = &job.blk.PHeight
	}

	// Fetch PChainHeight for this epoch
	//
	// We don't need to check timestamps here becuase we already handled that in block verification.
	epoch := utils.Epoch(job.blk.StatefulBlock.Timestamp, r.GetEpochDuration())
	_, epochHeights, err := e.GetEpochHeights(ctx, []uint64{epoch, epoch + 1})
	if err != nil {
		return err
	}

	// Process chunks
	//
	// We know that if any new available chunks are added that block context must be non-nil (so warp messages will be processed).
	p := NewProcessor(e.vm, e, pHeight, epochHeights, len(job.blk.AvailableChunks), job.blk.StatefulBlock.Timestamp, e.db, r)
	chunks := make([]*Chunk, 0, len(job.blk.AvailableChunks))
	for chunk := range job.chunks {
		// Handle fetched chunk
		p.Add(ctx, len(chunks), chunk)
		chunks = append(chunks, chunk)
	}
	txSet, ts, chunkResults, err := p.Wait()
	if err != nil {
		e.vm.Logger().Fatal("chunk processing failed", zap.Error(err)) // does not actually panic
		return err
	}
	if len(chunks) != len(job.blk.AvailableChunks) {
		// This can happen on the shutdown path. If this is because of an error, the chunk manager will FATAL.
		e.vm.Logger().Warn("did not receive all chunks from engine, exiting execution")
		return ErrMissingChunks
	}
	e.vm.RecordExecutedChunks(len(chunks))

	// TODO: pay beneficiary all tips for processing chunk
	//
	// TODO: add configuration for how much of base fee to pay (could be 100% in a payment network, however,
	// this allows miner to drive up fees without consequence as they get their fees back)

	// Create FilteredChunks
	//
	// TODO: send chunk material here as soon as processed to speed up confirmation latency
	txCount := 0
	filteredChunks := make([]*FilteredChunk, len(chunkResults))
	for i, chunkResult := range chunkResults {
		var (
			validResults = make([]*Result, 0, len(chunkResult))
			chunk        = chunks[i]
			cert         = job.blk.AvailableChunks[i]
			txs          = make([]*Transaction, 0, len(chunkResult))
			invalidTxs   = make([]ids.ID, 0, 4) // TODO: make a const
			reward       uint64

			warpResults set.Bits64
			warpCount   uint
		)
		for j, txResult := range chunkResult {
			txCount++
			tx := chunk.Txs[j]
			if !txResult.Valid {
				txID := tx.ID()
				invalidTxs = append(invalidTxs, txID)
				// Remove txID from txSet if it was invalid and
				// it was the first txID of its kind seen in the block.
				if bl, ok := txSet[txID]; ok {
					if bl.chunk == i && bl.index == j {
						delete(txSet, txID)
					}
				}

				// TODO: handle case where Freezable (claim bond from user + freeze user)
				// TODO: need to ensure that mark tx in set to prevent freezable replay?
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

			// Add fee to reward
			newReward, err := math.Add64(reward, txResult.Fee)
			if err != nil {
				panic(err)
			}
			reward = newReward
		}

		// TODO: Pay beneficiary proportion of reward
		// TODO: scale reward based on % of stake that signed cert
		p.vm.Logger().Debug("rewarding beneficiary", zap.Uint64("reward", reward))

		// Create filtered chunk
		filteredChunks[i] = &FilteredChunk{
			Chunk: cert.Chunk,

			Producer:    chunk.Producer,
			Beneficiary: chunk.Beneficiary,

			Txs:         txs,
			WarpResults: warpResults,
		}

		// As soon as execution of transactions is finished, let the VM know so that it
		// can notify subscribers.
		e.vm.ExecutedChunk(ctx, job.blk.StatefulBlock, filteredChunks[i], validResults, invalidTxs) // handled async by the vm
		e.vm.RecordTxsInvalid(len(invalidTxs))
	}

	// Update tracked p-chain height as long as it is increasing
	metaChunk := len(chunks)
	ts.PrepareChunk(metaChunk, 1)
	tsv := ts.NewWriteView(metaChunk, 0)
	if job.blk.PHeight > 0 { // if context is not set, don't update P-Chain height in state or populate epochs
		if shouldUpdatePHeight {
			if err := tsv.Put(ctx, pHeightKey, binary.BigEndian.AppendUint64(nil, job.blk.PHeight)); err != nil {
				panic(err)
			}
			e.vm.Logger().Info("setting current p-chain height", zap.Uint64("height", job.blk.PHeight))
		} else {
			e.vm.Logger().Debug("ignoring p-chain height update", zap.Uint64("height", job.blk.PHeight))
		}

		// Ensure we are never stuck waiting for height information near the end of an epoch
		//
		// When validating data in a given epoch e, we need to know the p-chain height for epoch e and e+1. If either
		// is not populated, we need to know by e that e+1 cannot be populated. If we only set e+2 below, we may
		// not know whether e+1 is populated unitl we wait for e-1 execution to finish (as any block in e-1 could set
		// the epoch height). This could cause verification to stutter across the boundary when it is taking longer than
		// expected to set to the p-chain hegiht for an epoch.
		nextEpoch := epoch + 3
		nextEpochKey := EpochKey(e.vm.StateManager().EpochKey(nextEpoch))
		epochValueRaw, err := e.db.Get(ctx, nextEpochKey) // <P-Chain Height>|<Fee Dimensions>
		switch {
		case err == nil:
			e.vm.Logger().Debug(
				"height already set for epoch",
				zap.Uint64("epoch", nextEpoch),
				zap.Uint64("height", binary.BigEndian.Uint64(epochValueRaw[:consts.Uint64Len])),
			)
		case err != nil && errors.Is(err, database.ErrNotFound):
			value := make([]byte, consts.Uint64Len)
			binary.BigEndian.PutUint64(value, job.blk.PHeight)
			if err := tsv.Put(ctx, nextEpochKey, value); err != nil {
				panic(err)
			}
			e.vm.CacheValidators(ctx, job.blk.PHeight) // optimistically fetch validators to prevent lockbacks
			e.vm.Logger().Info(
				"setting epoch height",
				zap.Uint64("epoch", nextEpoch),
				zap.Uint64("height", job.blk.PHeight),
			)
		default:
			e.vm.Logger().Warn(
				"unable to determine if should set epoch height",
				zap.Uint64("epoch", nextEpoch),
				zap.Error(err),
			)
		}
	}

	// Update chain metadata
	if err := tsv.Put(ctx, heightKey, binary.BigEndian.AppendUint64(nil, job.blk.StatefulBlock.Height)); err != nil {
		return err
	}
	if err := tsv.Put(ctx, HeightKey(e.vm.StateManager().TimestampKey()), binary.BigEndian.AppendUint64(nil, uint64(job.blk.StatefulBlock.Timestamp))); err != nil {
		return err
	}
	tsv.Commit()

	// Persist changes to state
	commitStart := time.Now()
	tstart := time.Now()
	openBytes, rewrite := batch.Prepare()
	e.vm.RecordVilmoBatchPrepare(time.Since(tstart))
	tstart = time.Now()
	changes := 0
	if err := ts.Iterate(func(key string, value maybe.Maybe[[]byte]) error {
		changes++
		if value.IsNothing() {
			return batch.Delete(context.TODO(), key)
		} else {
			return batch.Put(context.TODO(), key, value.Value())
		}
	}); err != nil {
		return err
	}
	e.vm.RecordTStateIterate(time.Since(tstart))
	tstart = time.Now()
	checksum, err := batch.Write()
	if err != nil {
		return err
	}
	e.vm.RecordVilmoBatchWrite(time.Since(tstart))
	e.vm.RecordStateChanges(changes)
	e.vm.RecordVilmoBatchInitBytes(openBytes)
	if rewrite {
		e.vm.RecordVilmoBatchesRewritten()
	}
	e.vm.RecordWaitCommit(time.Since(commitStart))

	// Store and update parent view
	validTxs := len(txSet)
	e.outputsLock.Lock()
	e.outputs[job.blk.StatefulBlock.Height] = &output{
		txs:          txSet,
		chunkResults: chunkResults,

		chunks:   filteredChunks,
		checksum: checksum,
	}
	e.largestOutput = &job.blk.StatefulBlock.Height
	e.outputsLock.Unlock()

	log.Info(
		"executed block",
		zap.Stringer("blkID", job.blk.ID()),
		zap.Uint64("height", job.blk.StatefulBlock.Height),
		zap.Int("valid txs", validTxs),
		zap.Int("total txs", txCount),
		zap.Int("chunks", len(filteredChunks)),
		zap.Uint64("epoch", epoch),
		zap.Duration("t", time.Since(estart)),
	)
	e.vm.RecordBlockExecute(time.Since(estart))
	e.vm.RecordTxsIncluded(txCount)
	e.vm.RecordExecutedEpoch(epoch)
	e.vm.ExecutedBlock(ctx, job.blk.StatefulBlock)
	return nil
}

func (e *Engine) Run() {
	defer close(e.done)

	// Get last accepted state
	e.db = e.vm.State()
	batchStart := time.Now()
	batch, err := e.db.NewBatch()
	if err != nil {
		panic(err)
	}
	e.vm.RecordVilmoBatchInit(time.Since(batchStart))

	for {
		select {
		case job := <-e.backlog:
			err := e.processJob(batch, job)
			switch {
			case err == nil:
				// Start preparation for new batch
				batchStart := time.Now()
				batch, err = e.db.NewBatch()
				if err != nil {
					panic(err)
				}
				e.vm.RecordVilmoBatchInit(time.Since(batchStart))
				continue
			case errors.Is(ErrMissingChunks, err):
				// Should only happen on shutdown
				e.vm.Logger().Warn("engine shutting down", zap.Error(err))
				return
			default:
				panic(err) // unrecoverable error
			}
		case <-e.vm.StopChan():
			// Give up latest batch
			batch.Abort()
			return
		}
	}
}

func (e *Engine) Execute(blk *StatelessBlock) {
	// Request chunks for processing when ready
	chunks := make(chan *Chunk, len(blk.AvailableChunks))
	e.vm.RequestChunks(blk.Height(), blk.AvailableChunks, chunks) // spawns a goroutine

	// Enqueue job
	e.vm.RecordEngineBacklog(1)
	e.backlog <- &engineJob{
		parentTimestamp: blk.parent.StatefulBlock.Timestamp,
		blk:             blk,
		chunks:          chunks,
	}
}

func (e *Engine) Results(height uint64) ([]ids.ID /* Executed Chunks */, ids.ID /* Checksum */, error) {
	// TODO: handle case where never started execution (state sync)
	e.outputsLock.RLock()
	defer e.outputsLock.RUnlock()

	if output, ok := e.outputs[height]; ok {
		filteredIDs := make([]ids.ID, len(output.chunks))
		for i, chunk := range output.chunks {
			id, err := chunk.ID()
			if err != nil {
				return nil, ids.Empty, err
			}
			filteredIDs[i] = id
		}
		return filteredIDs, output.checksum, nil
	}
	return nil, ids.Empty, fmt.Errorf("%w: results not found for %d", errors.New("not found"), height)
}

func (e *Engine) PruneResults(ctx context.Context, height uint64) ([][]*Result, []*FilteredChunk, error) {
	e.outputsLock.Lock()
	defer e.outputsLock.Unlock()

	output, ok := e.outputs[height]
	if !ok {
		return nil, nil, fmt.Errorf("%w: %d", errors.New("not outputs found at height"), height)
	}
	delete(e.outputs, height)
	if e.largestOutput != nil && *e.largestOutput == height {
		e.largestOutput = nil
	}
	return output.chunkResults, output.chunks, nil
}

func (e *Engine) ReadLatestState(ctx context.Context, keys []string) ([][]byte, []error) {
	return e.db.Gets(ctx, keys)
}

func (e *Engine) Done() {
	<-e.done
}

func (e *Engine) GetEpochHeights(ctx context.Context, epochs []uint64) (int64, []*uint64, error) {
	keys := []string{HeightKey(e.vm.StateManager().TimestampKey())}
	for _, epoch := range epochs {
		keys = append(keys, EpochKey(e.vm.StateManager().EpochKey(epoch)))
	}
	values, errs := e.ReadLatestState(ctx, keys)
	if errs[0] != nil {
		return -1, nil, fmt.Errorf("%w: can't read timestamp key", errs[0])
	}
	heights := make([]*uint64, len(epochs))
	for i := 0; i < len(epochs); i++ {
		if errs[i+1] != nil {
			continue
		}
		value := values[i+1]
		height := binary.BigEndian.Uint64(value[:consts.Uint64Len])
		heights[i] = &height
	}
	return int64(binary.BigEndian.Uint64(values[0])), heights, nil
}
