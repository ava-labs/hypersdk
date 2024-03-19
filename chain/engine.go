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
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

// TODO: unify with chunk wrapper
type simpleChunkWrapper struct {
	chunk   ids.ID
	slot    int64
	success bool
}

func (scw *simpleChunkWrapper) ID() ids.ID {
	return scw.chunk
}

func (scw *simpleChunkWrapper) Expiry() int64 {
	return scw.slot
}

type engineJob struct {
	parentTimestamp int64
	blk             *StatelessBlock
	chunks          chan *Chunk
}

type output struct {
	txs          map[ids.ID]*blockLoc
	chunkResults [][]*Result

	chunks []*FilteredChunk
	root   ids.ID
}

// Engine is in charge of orchestrating the execution of
// accepted chunks.
//
// TODO: put in VM?
type Engine struct {
	vm   VM
	done chan struct{}

	backlog chan *engineJob

	latestView state.View

	outputsLock   sync.RWMutex
	outputs       map[uint64]*output
	largestOutput *uint64

	authorized *eheap.ExpiryHeap[*simpleChunkWrapper]
}

func NewEngine(vm VM, maxBacklog int) *Engine {
	// TODO: strategically use old timestamps on blocks when catching up after processing
	// surge to maximize chunk inclusion
	return &Engine{
		vm:   vm,
		done: make(chan struct{}),

		backlog: make(chan *engineJob, maxBacklog),

		outputs: make(map[uint64]*output),

		authorized: eheap.New[*simpleChunkWrapper](64),
	}
}

func (e *Engine) processJob(job *engineJob) {
	log := e.vm.Logger()
	e.vm.RecordEngineBacklog(-1)

	estart := time.Now()
	ctx := context.Background() // TODO: cleanup

	// Setup state
	parentView := e.latestView
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

	// Fetch latest block context (used for reliable and recent warp verification)
	var (
		pHeight             *uint64
		shouldUpdatePHeight bool
	)
	pHeightKey := PHeightKey(e.vm.StateManager().PHeightKey())
	pHeightRaw, err := parentView.GetValue(ctx, pHeightKey)
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
		panic(err)
	}

	// Process chunks
	//
	// We know that if any new available chunks are added that block context must be non-nil (so warp messages will be processed).
	startProcessor := time.Now()
	p := NewProcessor(e.vm, e, pHeight, epochHeights, len(job.blk.AvailableChunks), job.blk.StatefulBlock.Timestamp, parentView, r)
	chunks := make([]*Chunk, 0, len(job.blk.AvailableChunks))
	for chunk := range job.chunks {
		// Handle fetched chunk
		cw, _ := e.authorized.Remove(chunk.ID())
		p.Add(ctx, len(chunks), chunk, cw)
		chunks = append(chunks, chunk)
	}
	uselessAuthorization := len(e.authorized.SetMin(job.blk.StatefulBlock.Timestamp)) // cleanup unneeded verification statuses
	if uselessAuthorization > 0 {
		e.vm.Logger().Warn("performed useless authorization", zap.Int("count", uselessAuthorization))
	}
	e.vm.RecordUnusedAuthorizedChunks(uselessAuthorization)
	txSet, ts, chunkResults, err := p.Wait()
	if err != nil {
		e.vm.Logger().Fatal("chunk processing failed", zap.Error(err))
		return
	}
	if len(chunks) != len(job.blk.AvailableChunks) {
		// This can happen on the shutdown path. If this is because of an error, the chunk manager will FATAL.
		e.vm.Logger().Warn("did not receive all chunks from engine, exiting execution")
		return
	}
	e.vm.RecordExecutedChunks(len(chunks))
	e.vm.RecordWaitProcessor(time.Since(startProcessor))

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
			reward       uint64

			warpResults set.Bits64
			warpCount   uint
		)
		for j, txResult := range chunkResult {
			txCount++
			tx := chunk.Txs[j]
			if !txResult.Valid {
				// Remove txID from txSet if it was invalid and
				// it was the first txID of its kind seen in the block.
				if bl, ok := txSet[tx.ID()]; ok {
					if bl.chunk == i && bl.index == j {
						delete(txSet, tx.ID())
					}
				}

				// TODO: handle case where Freezable (claim bond from user + freeze user)
				// TODO: need to ensure that mark tx in set to prevent freezable replay?

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
		e.vm.ExecutedChunk(ctx, job.blk.Height(), filteredChunks[i], validResults) // handled async by the vm
	}

	// Update tracked p-chain height as long as it is increasing
	if job.blk.PHeight > 0 { // if context is not set, don't update P-Chain height in state or populate epochs
		if shouldUpdatePHeight {
			if err := ts.Insert(ctx, pHeightKey, binary.BigEndian.AppendUint64(nil, job.blk.PHeight)); err != nil {
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
		epochValueRaw, err := parentView.GetValue(ctx, nextEpochKey) // <P-Chain Height>|<Fee Dimensions>
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
			if err := ts.Insert(ctx, nextEpochKey, value); err != nil {
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
	if err := ts.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, job.blk.StatefulBlock.Height)); err != nil {
		panic(err)
	}
	if err := ts.Insert(ctx, HeightKey(e.vm.StateManager().TimestampKey()), binary.BigEndian.AppendUint64(nil, uint64(job.blk.StatefulBlock.Timestamp))); err != nil {
		panic(err)
	}

	// Create new view and persist to disk
	e.vm.RecordStateChanges(ts.PendingChanges())
	e.vm.RecordStateOperations(ts.OpIndex())
	view, err := ts.ExportMerkleDBView(ctx, e.vm.Tracer(), parentView)
	if err != nil {
		panic(err)
	}
	commitStart := time.Now()
	if err := view.CommitToDB(ctx); err != nil {
		panic(err)
	}
	root, err := view.GetMerkleRoot(ctx)
	if err != nil {
		panic(err)
	}
	e.vm.RecordWaitCommit(time.Since(commitStart))

	// Store and update parent view
	validTxs := len(txSet)
	e.outputsLock.Lock()
	e.outputs[job.blk.StatefulBlock.Height] = &output{
		txs:          txSet,
		chunkResults: chunkResults,
		root:         root,
		chunks:       filteredChunks,
	}
	e.largestOutput = &job.blk.StatefulBlock.Height
	e.outputsLock.Unlock()
	e.latestView = state.View(e.vm.ForceState()) // Don't offer views that will be discarded

	log.Info(
		"executed block",
		zap.Stringer("blkID", job.blk.ID()),
		zap.Uint64("height", job.blk.StatefulBlock.Height),
		zap.Int("valid txs", validTxs),
		zap.Int("total txs", txCount),
		zap.Int("chunks", len(filteredChunks)),
		zap.Stringer("root", root),
		zap.Duration("t", time.Since(estart)),
	)
	e.vm.RecordBlockExecute(time.Since(estart))
	e.vm.RecordTxsInvalid(txCount - validTxs)
	e.vm.RecordTxsIncluded(txCount)
	e.vm.ExecutedBlock(ctx, job.blk)
}

func (e *Engine) Run() {
	defer close(e.done)

	// Get last accepted state
	e.latestView = state.View(e.vm.ForceState()) // TODO: state may not be ready at this point

	for {
		// Check to see if anything is on the backlog or if we should stop
		select {
		case job := <-e.backlog:
			e.processJob(job)
			continue
		case <-e.vm.StopChan():
			return
		default:
		}

		// Check to see if any chunks are ready for optimistic signature verification (if there is nothing else to do)
		select {
		case cert := <-e.vm.CertChan():
			// Need to ensure this stream is deduped (can't just send as soon as a chunk is ready)
			if e.vm.IsSeenChunk(context.TODO(), cert.Chunk) {
				// Will process during execution loop or already processed
				continue
			}
			chunk, err := e.vm.GetChunk(cert.Slot, cert.Chunk)
			if chunk == nil || err != nil {
				e.vm.Logger().Warn("chunk not available for optimistic verification", zap.Stringer("chunkID", cert.Chunk), zap.Error(err))
				continue
			}
			if e.vm.NodeID() == chunk.Producer {
				// Don't verify own chunks
				continue
			}
			if !e.vm.GetVerifyAuth() {
				// Don't verify chunks if not verifying
				continue
			}
			start := time.Now()
			result := AuthorizeChunk(e.vm, chunk)
			e.authorized.Add(&simpleChunkWrapper{
				chunk:   cert.Chunk,
				slot:    cert.Slot,
				success: result,
			})
			dur := time.Since(start)
			e.vm.Logger().Debug("optimistically authorized chunk", zap.Stringer("chunkID", cert.Chunk), zap.Int("txs", len(chunk.Txs)), zap.Bool("success", result), zap.Duration("t", dur))
			e.vm.RecordOptimisticAuthorizedChunk(dur)
		case job := <-e.backlog:
			e.processJob(job)
		case <-e.vm.StopChan():
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

func (e *Engine) Results(height uint64) (ids.ID /* Root */, []ids.ID /* Executed Chunks */, error) {
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
		return output.root, filteredIDs, nil
	}
	return ids.Empty, nil, fmt.Errorf("%w: results not found for %d", errors.New("not found"), height)
}

// TODO: cleanup this function signautre
func (e *Engine) Commit(ctx context.Context, height uint64) ([][]*Result, []*FilteredChunk, error) {
	// TODO: fetch results prior to commit to reduce observed finality (state won't be queryable yet but can send block/results)
	e.outputsLock.Lock()
	output, ok := e.outputs[height]
	if !ok {
		e.outputsLock.Unlock()
		return nil, nil, fmt.Errorf("%w: %d", errors.New("not outputs found at height"), height)
	}
	delete(e.outputs, height)
	if e.largestOutput != nil && *e.largestOutput == height {
		e.largestOutput = nil
	}
	e.outputsLock.Unlock()
	return output.chunkResults, output.chunks, nil
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

func (e *Engine) ReadLatestState(ctx context.Context, keys [][]byte) ([][]byte, []error) {
	view := e.latestView
	results := make([][]byte, len(keys))
	errors := make([]error, len(keys))
	for i, key := range keys {
		result, err := view.GetValue(ctx, key)
		results[i] = result
		errors[i] = err
	}
	return results, errors
}

func (e *Engine) Done() {
	<-e.done
}

func (e *Engine) GetEpochHeights(ctx context.Context, epochs []uint64) (int64, []*uint64, error) {
	keys := [][]byte{HeightKey(e.vm.StateManager().TimestampKey())}
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
