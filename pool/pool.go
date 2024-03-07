package pool

import (
	"sync"
	"sync/atomic"
)

type Pool struct {
	workerCount        atomic.Int32
	workerSpawner      chan struct{}
	outstandingWorkers sync.WaitGroup

	work      chan func()
	workClose sync.Once
}

// New returns an instance of [Pool] that
// will spawn up to [max] goroutines.
func New(max int) *Pool {
	return &Pool{
		workerSpawner: make(chan struct{}, max),
		work:          make(chan func()),
	}
}

// startWorker creates a new goroutine to execute [f] immediately and then keeps the goroutine
// alive to continue executing new work.
func (p *Pool) startWorker(f func()) {
	p.workerCount.Add(1)
	p.outstandingWorkers.Add(1)

	go func() {
		defer p.outstandingWorkers.Done()

		if f != nil {
			f()
		}
		for f := range p.work {
			f()
		}
	}()
}

// Execute the given function on an existing goroutine waiting for more work, a new goroutine,
// or return if the context is canceled.
//
// Execute must not be called after Wait, otherwise it might panic.
func (p *Pool) Execute(f func()) {
	// Ensure we feed idle workers first
	select {
	case p.work <- f:
		return
	default:
	}

	// Fallback to waiting for an idle worker or allocating
	// a new worker (if we aren't yet at max concurrency)
	select {
	case p.work <- f:
	case p.workerSpawner <- struct{}{}:
		p.startWorker(f)
	}
}

// Wait returns after all enqueued work finishes and all goroutines to exit.
// Wait returns the number of workers that were spawned during the run.
//
// Wait can only be called after ALL calls to [Execute] have returned.
//
// It is safe to call Wait multiple times but not safe to call [Execute]
// after [Wait] has been called.
func (p *Pool) Wait() int {
	p.workClose.Do(func() {
		close(p.work)
	})
	p.outstandingWorkers.Wait()
	return int(p.workerCount.Load())
}
