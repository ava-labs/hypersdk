// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/state"
)

const (
	defaultSetSize = 8
	notSet         = -1
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

// We can process up to 1 Allocate/Write and many reads
type data struct {
	allocateWrite int
	reads         set.Set[int]
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

	dependencies set.Set[int]
	blocking     set.Set[int]

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
			// Update concurrent reads to only contain not-yet-executed reads
			for k := range t.keys {
				key := e.edges[k]
				key.reads.Remove(t.id)
			}
			for b := range t.blocking { // works fine on non-initialized map
				bt := e.tasks[b]
				bt.dependencies.Remove(t.id)
				if bt.dependencies.Len() == 0 { // must be non-nil to be blocked
					bt.dependencies = nil // free memory
					e.executable <- bt
				}
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
// overlapping [conflicts] are executed.
func (e *Executor) Run(conflicts state.Keys, f func() error) {
	e.l.Lock()
	defer e.l.Unlock()

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
			// No Allocate/Write has been requested yet
			if key.allocateWrite == notSet {
				if v == state.Read {
					key.reads.Add(id)
				} else {
					key.allocateWrite = id
					for b := range key.reads {
						bt := e.tasks[b]
						recordDependencies(t, bt)
					}
				}
				continue
			}

			// Get last blocked Allocate/Write task
			lt := e.tasks[key.allocateWrite]
			if !lt.executed {
				if t.dependencies == nil {
					t.dependencies = set.NewSet[int](defaultSetSize)
				}
				if lt.blocking == nil {
					lt.blocking = set.NewSet[int](defaultSetSize)
				}

				if v == state.Read {
					key.reads.Add(id)
					recordDependencies(t, lt)
					continue
				}

				if key.reads.Len() == 0 {
					recordDependencies(t, lt)
				} else {
					for b := range key.reads {
						bt := e.tasks[b]
						recordDependencies(t, bt)
					}
				}
			}
		}
		keyData := &data{allocateWrite: notSet, reads: set.Set[int]{}}
		if v == state.Read {
			keyData.reads.Add(id)
		} else {
			keyData.allocateWrite = id
		}
		e.edges[k] = keyData
	}

	// Start execution if there are no blocking dependencies
	if t.dependencies == nil || t.dependencies.Len() == 0 {
		t.dependencies = nil // free memory
		e.executable <- t
		if e.metrics != nil {
			e.metrics.RecordExecutable()
		}
		return
	}
	if e.metrics != nil {
		e.metrics.RecordBlocked()
	}
}

func recordDependencies(t *task, bt *task) {
	t.dependencies.Add(bt.id)
	bt.blocking.Add(t.id)
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
