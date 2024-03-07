package pool

import (
	"sync"
	"sync/atomic"
)

type task struct {
	i int
	f func() func()
}

type Pool struct {
	workerCount        atomic.Int32
	workerSpawner      chan struct{}
	outstandingWorkers sync.WaitGroup

	enqueued  int
	work      chan *task
	workClose sync.Once

	processedL sync.Mutex
	processedM map[int]func()
	toProcess  int
}

// New returns an instance of [Pool] that
// will spawn up to [max] goroutines.
func New(maxWorkers int) *Pool {
	return &Pool{
		workerSpawner: make(chan struct{}, maxWorkers),
		work:          make(chan *task),
		processedM:    make(map[int]func()),
	}
}

func (p *Pool) runTask(t *task) {
	f := t.f()

	// Run available functions
	funcs := []func(){}
	p.processedL.Lock()
	if t.i != p.toProcess {
		p.processedM[t.i] = f
	} else {
		funcs = append(funcs, f)
		p.toProcess++
		for {
			if f, ok := p.processedM[p.toProcess]; ok {
				funcs = append(funcs, f)
				delete(p.processedM, p.toProcess)
				p.toProcess++
			} else {
				break
			}
		}
	}
	// We must execute these functions with the lock held
	// to ensure they are executed in the correct order.
	for _, f := range funcs {
		if f != nil {
			f()
		}
	}
	p.processedL.Unlock()
}

// startWorker creates a new goroutine to execute [f] immediately and then keeps the goroutine
// alive to continue executing new work.
func (p *Pool) startWorker(t *task) {
	p.workerCount.Add(1)
	p.outstandingWorkers.Add(1)

	go func() {
		defer p.outstandingWorkers.Done()

		p.runTask(t)
		for nt := range p.work {
			p.runTask(nt)
		}
	}()
}

// Go executes the given function on an existing goroutine waiting for more work or spawn
// a new goroutine. If [f] returns a function, it will be executed in the order
// it was enqueued. Returned functions are executed serially by the pool (blocking
// the further computation of a worker until complete), so it is important to
// minimize the returned function complexity.
//
// Go must not be called after Wait, otherwise it might panic.
//
// Go should not be called concurrently from multiple goroutines.
func (p *Pool) Go(f func() func()) {
	t := &task{i: p.enqueued, f: f}
	p.enqueued++

	// Ensure we feed idle workers first
	select {
	case p.work <- t:
		return
	default:
	}

	// Fallback to waiting for an idle worker or allocating
	// a new worker (if we aren't yet at max concurrency)
	select {
	case p.work <- t:
	case p.workerSpawner <- struct{}{}:
		p.startWorker(t)
	}
}

// Wait returns after all enqueued work finishes and all goroutines to exit.
// Wait returns the number of workers that were spawned during the run.
//
// Wait can only be called after ALL calls to [Execute] have returned.
//
// It is safe to call Wait multiple times but not safe to call [Execute]
// after [Wait] has been called.
func (p *Pool) Wait() (int, error) {
	p.workClose.Do(func() {
		close(p.work)
	})
	p.outstandingWorkers.Wait()
	return int(p.workerCount.Load()), nil
}
