// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"
	"sync/atomic"

	// "github.com/ava-labs/avalanchego/utils/set"

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
	nodes     map[string]int
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		stop:       make(chan struct{}),
		tasks:      make(map[int]*task, items),
		nodes:      make(map[string]int, items*2), // TODO: tune this
		executable: make(chan *task, items),       // ensure we don't block while holding lock
	}
	e.wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go e.runWorker()
	}
	return e
}

type task struct {
	id int
	f  func() error

	dependencies atomic.Int64
	blocking     map[int]*task

	executed bool
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
			for _, bt := range t.blocking {
				if bt.dependencies.Add(-1) > 0 {
					continue
				}
				e.executable <- bt
			}
			t.blocking = nil // free memory
			t.executed = true
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
// overlapping [keys] are executed.
func (e *Executor) Run(keys state.Keys, f func() error) {
	e.l.Lock()
	defer e.l.Unlock()

	// Add task to map
	id := len(e.tasks)
	t := &task{
		id:       id,
		f:        f,
		blocking: map[int]*task{},
	}
	e.tasks[id] = t

	// Record dependencies
	//previousDependencies := set.NewSet[int](len(keys))
	for k, v := range keys {
		latest, ok := e.nodes[k]
		if ok {
			lt := e.tasks[latest]
			if !lt.executed {
				if v.Has(state.Allocate) || v.Has(state.Write) {
					t.dependencies.Add(int64(1))
				}
				lt.blocking[id] = t
			}
		}
		e.nodes[k] = id
	}

	if t.dependencies.Load() > 0 {
		if e.metrics != nil {
			e.metrics.RecordBlocked()
		}
		return
	}

	// Mark task for execution if we aren't waiting on any other tasks
	e.executable <- t
	if e.metrics != nil {
		e.metrics.RecordExecutable()
	}
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
