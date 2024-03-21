package vm

import (
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
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

type ChunkAuthorizer struct {
	vm          *VM
	authWorkers workers.Workers

	jobsL sync.Mutex
	jobs  map[ids.ID]*job

	required   chan ids.ID
	optimistic chan ids.ID
}

func NewChunkAuthorizer(vm *VM) *ChunkAuthorizer {
	return &ChunkAuthorizer{
		vm:         vm,
		jobs:       make(map[ids.ID]*job, 128),
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
			return
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
	c.jobsL.Lock()
	result := c.jobs[id]
	c.jobsL.Unlock()
	if result.result != nil {
		// Can happen if a cert is in [required] and [optimistic]
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
			result.chunk = nil
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
	result.chunk = nil
	close(result.done)
}

// It is safe to call [Add] multiple times with the same chunk
func (c *ChunkAuthorizer) Add(chunk *chain.Chunk, cert *chain.ChunkCertificate) {
	// Check if already added
	c.jobsL.Lock()
	if _, ok := c.jobs[cert.Chunk]; ok {
		c.jobsL.Unlock()
		return
	}
	c.jobs[cert.Chunk] = &job{
		chunk: chunk,
		done:  make(chan struct{}),
	}
	c.jobsL.Unlock()

	// Queue chunk for authorization
	c.optimistic <- cert.Chunk
}

func (c *ChunkAuthorizer) Wait(id ids.ID) bool {
	c.jobsL.Lock()
	result, ok := c.jobs[id]
	c.jobsL.Unlock()
	if !ok {
		panic("waiting on certificate that wasn't enqueued")
	}

	// If cert auth is not done yet, make sure to prioritize it
	if result.result == nil {
		c.required <- id
	}

	// Wait for result
	<-result.done
	return *result.result
}

func (c *ChunkAuthorizer) Remove(certs []ids.ID) {
	c.jobsL.Lock()
	defer c.jobsL.Unlock()

	for _, id := range certs {
		delete(c.jobs, id)
	}
}
