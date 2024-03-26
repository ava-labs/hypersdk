// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"

	"github.com/ava-labs/hypersdk/state"
)

// Executor sequences the concurrent execution of
// tasks with arbitrary conflicts on-the-fly.
//
// Executor ensures that conflicting tasks
// are executed in the order they were queued.
// Tasks with no conflicts are executed immediately.
type Executor struct {
	metrics    Metrics
	wg         sync.WaitGroup
	executable chan *task

	stop     chan struct{}
	err      error
	stopOnce sync.Once

	l         sync.Mutex
	done      bool
	completed int
	tasks     map[int]*task
	edges     map[string]*data
}

// invariant: [data] holds either 1 Allocate/Write or
// a set of Reads, but never both
//
// invariant: [waiter] is made every time when setting
// Allocate/Write or when adding the first Read. [waiter]
// is closed only when [blockers] == 0
type data struct {
	isAllocateWrite bool

	waiter   chan struct{}
	blockers int
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		stop:       make(chan struct{}),
		tasks:      make(map[int]*task, items),
		edges:      make(map[string]*data, items*2), // TODO: tune this
		executable: make(chan *task, items),         // ensure we don't block while holding lock
	}
	e.wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go e.runWorker()
	}
	return e
}

type task struct {
	id   int
	f    func() error
	keys state.Keys
}

func (e *Executor) runWorker() {
	defer e.wg.Done()

	for {
		select {
		case t, ok := <-e.executable:
			if !ok {
				return
			}

			if err := t.f(); err != nil {
				e.stopOnce.Do(func() {
					e.err = err
					close(e.stop)
				})
				return
			}

			e.l.Lock()
			for k := range t.keys {
				key := e.edges[k]
				key.blockers--
				if key.blockers == 0 {
					close(key.waiter)
					// Allows us to always make a [waiter] chan
					// after executing the last blocked task.
					delete(e.edges, k)
				}
			}
			e.completed++
			if e.done && e.completed == len(e.tasks) {
				// We will close here if there are unexecuted tasks
				// when we call [Wait].
				close(e.executable)
			}
			e.l.Unlock()
		case <-e.stop:
			return
		}
	}
}

// Run executes [f] after all previously enqueued [f] with
// overlapping [conflicts] are executed.
func (e *Executor) Run(conflicts state.Keys, f func() error) {
	e.l.Lock()

	// Add task to map
	id := len(e.tasks)
	t := &task{
		id:   id,
		f:    f,
		keys: conflicts,
	}
	e.tasks[id] = t

	// Record dependencies
	for k, v := range conflicts {
		key, ok := e.edges[k]
		if ok {
			// Keep processing more Reads
			if v == state.Read && !key.isAllocateWrite {
				key.blockers++
				continue
			}

			// Block on write-after-write, read-after-write, or write-after-reads
			if e.metrics != nil {
				e.metrics.RecordBlocked()
			}
			e.l.Unlock()
			<-key.waiter
			e.l.Lock()
		}
		// Key doesn't exist or we just processed Allocate/Write or many Reads
		e.edges[k] = &data{
			isAllocateWrite: v.Has(state.Allocate) || v.Has(state.Write),
			waiter:          make(chan struct{}),
			blockers:        1,
		}
	}
	if e.metrics != nil {
		e.metrics.RecordExecutable()
	}
	e.l.Unlock()
	e.executable <- t
}

func (e *Executor) Stop() {
	e.stopOnce.Do(func() {
		e.err = ErrStopped
		close(e.stop)
	})
}

// Wait returns as soon as all enqueued [f] are executed.
//
// You should not call [Run] after [Wait] is called.
func (e *Executor) Wait() error {
	e.l.Lock()
	e.done = true
	if e.completed == len(e.tasks) {
		// We will close here if all tasks
		// are executed by the time we call [Wait].
		close(e.executable)
	}
	e.l.Unlock()
	e.wg.Wait()
	return e.err
}
