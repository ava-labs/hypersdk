package opool

import (
	"sync"

	"go.uber.org/atomic"
)

// OPool is a pool of goroutines that can execute functions in parallel
// but invoke callbacks (if provided) in the order they are enqueued.
type OPool struct {
	added int

	work      chan *task
	workClose sync.Once

	outstandingWorkers sync.WaitGroup

	processedL sync.Mutex
	processedM map[int]func()
	toProcess  int

	err atomic.Error
}

// New returns an instance of [OPool] that
// has [worker] goroutines.
func New(workers, backlog int) *OPool {
	p := &OPool{
		work:       make(chan *task, backlog),
		processedM: make(map[int]func()),
	}
	for i := 0; i < workers; i++ {
		// We assume all workers will be required and start
		// them immediately.
		p.startWorker()
	}
	return p
}

func (p *OPool) startWorker() {
	p.outstandingWorkers.Add(1)

	go func() {
		defer p.outstandingWorkers.Done()

		for t := range p.work {
			p.run(t)
		}
	}()
}

type task struct {
	i int
	f func() (func(), error)
}

func (p *OPool) run(t *task) {
	if p.err.Load() != nil {
		return
	}

	f, err := t.f()
	if err != nil {
		p.err.CompareAndSwap(nil, err)
		return
	}

	// Run available functions
	p.processedL.Lock()
	defer p.processedL.Unlock()
	funcs := []func(){}
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
		if f == nil {
			continue
		}
		f()
	}
}

// Go executes the given function on an existing goroutine waiting for more work. If
// [f] returns a function, it will be executed in the order it was enqueued. Returned
// functions are executed by the worker directly (blocking processing of other callbacks
// and other tasks by the worker), so it is important to minimize the returned function complexity.
//
// Go must not be called after Wait, otherwise it might panic.
//
// Go should not be called concurrently from multiple goroutines.
//
// If the pool has errored, Go will not execute the function and will return immediately.
// This means that enqueued functions may never be executed.
func (p *OPool) Go(f func() (func(), error)) {
	if p.err.Load() != nil {
		return
	}

	t := &task{i: p.added, f: f}
	p.added++
	p.work <- t
}

// Wait returns after all enqueued work finishes and all goroutines to exit.
// Wait returns the number of workers that were spawned during the run.
//
// Wait can only be called after ALL calls to [Execute] have returned.
//
// It is safe to call Wait multiple times but not safe to call [Execute]
// after [Wait] has been called.
func (p *OPool) Wait() error {
	p.workClose.Do(func() {
		close(p.work)
	})
	p.outstandingWorkers.Wait()
	return p.err.Load()
}
