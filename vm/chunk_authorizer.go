package vm

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/workers"
	"go.uber.org/zap"
)

var (
	t        = true
	f        = false
	truePtr  = &t
	falsePtr = &f
)

type job struct {
	chunk  *chain.Chunk
	result *bool
	done   chan struct{}
}

func (j *job) ID() ids.ID    { return j.chunk.ID() }
func (j *job) Expiry() int64 { return j.chunk.Slot }

type ChunkAuthorizer struct {
	vm          *VM
	authWorkers workers.Workers

	jobs *eheap.ExpiryHeap[*job]

	required   chan ids.ID
	optimistic chan ids.ID
}

func NewChunkAuthorizer(vm *VM) *ChunkAuthorizer {
	return &ChunkAuthorizer{
		vm:         vm,
		jobs:       eheap.New[*job](128),
		required:   make(chan ids.ID, 128),
		optimistic: make(chan ids.ID, 128),
	}
}

func (c *ChunkAuthorizer) Run() {
	c.authWorkers = workers.NewParallel(c.vm.GetAuthExecutionCores(), 4)
	defer c.authWorkers.Stop()

	for {
		// Exit if the VM is shutting down
		select {
		case <-c.vm.stop:
			return
		default:
		}

		// Check if there are any jobs that should be authorized required
		select {
		case id := <-c.required:
			c.auth(id)
			continue
		default:
		}

		// Wait for either a priortized cert or any optimistic cert
		select {
		case id := <-c.required:
			c.auth(id)
		case id := <-c.optimistic:
			c.auth(id)
		case <-c.vm.stop:
			return
		}
	}
}

func (c *ChunkAuthorizer) auth(id ids.ID) {
	result, ok := c.jobs.Get(id)
	if !ok {
		// This can happen if chunk is expired before we get to it
		c.vm.Logger().Debug("skipping missing job", zap.Stringer("chunkID", id))
		return
	}
	if result.result != nil {
		// Can happen if a cert is in [required] and [optimistic]
		return
	}
	if !c.vm.config.GetVerifyAuth() || c.vm.snowCtx.NodeID == result.chunk.Producer { // trust ourself
		result.result = truePtr
		close(result.done)
		return
	}

	// Record time it takes to authorize the chunk
	start := time.Now()
	defer func() {
		c.vm.metrics.chunkAuth.Observe(float64(time.Since(start)))
	}()

	chunk := result.chunk
	authJob, err := c.authWorkers.NewJob(len(chunk.Txs))
	if err != nil {
		panic(err)
	}
	batchVerifier := chain.NewAuthBatch(c.vm, authJob, chunk.AuthCounts())
	for _, tx := range chunk.Txs {
		// Enqueue transaction for execution
		msg, err := tx.Digest()
		if err != nil {
			c.vm.Logger().Error("chunk failed auth", zap.Stringer("chunk", chunk.ID()), zap.Error(err))
			result.result = falsePtr
			close(result.done)
			return
		}
		if c.vm.IsRPCAuthorized(tx.ID()) {
			c.vm.RecordRPCAuthorizedTx()
			continue
		}

		// We can only pre-check transactions that would invalidate the chunk prior to verifying signatures.
		batchVerifier.Add(msg, tx.Auth)
	}
	batchVerifier.Done(nil)
	if err := authJob.Wait(); err != nil {
		c.vm.Logger().Error("chunk failed auth", zap.Stringer("chunk", chunk.ID()), zap.Error(err))
		result.result = falsePtr
	} else {
		c.vm.Logger().Debug("chunk authorized", zap.Stringer("chunk", chunk.ID()))
		result.result = truePtr
	}
	close(result.done)
}

// It is safe to call [Add] multiple times with the same chunk
func (c *ChunkAuthorizer) Add(chunk *chain.Chunk) {
	// Check if already added
	c.jobs.Add(&job{
		chunk: chunk,
		done:  make(chan struct{}),
	})

	// Queue chunk for authorization
	c.optimistic <- chunk.ID()
}

// It is not safe to call [Wait] multiple times with the same chunk or to call
// it concurrently.
func (c *ChunkAuthorizer) Wait(id ids.ID) bool {
	result, ok := c.jobs.Get(id)
	if !ok {
		// This panic would also catch if a chunk was included twice
		panic("waiting on certificate that wasn't enqueued")
	}

	// If cert auth is not done yet, make sure to prioritize it
	c.required <- id

	// Wait for result
	<-result.done

	// Remove chunk from tracking, it will never be requested again
	c.jobs.Remove(id)
	return *result.result
}

func (c *ChunkAuthorizer) SetMin(t int64) []ids.ID {
	elems := c.jobs.SetMin(t)
	items := make([]ids.ID, len(elems))
	for i, elem := range elems {
		items[i] = elem.ID()
	}
	return items
}
