// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"
	"sync/atomic"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/state"

	uatomic "go.uber.org/atomic"
)

// Executor sequences the concurrent execution of
// tasks with arbitrary conflicts on-the-fly.
//
// Executor ensures that conflicting tasks
// are executed in the order they were queued.
// Tasks with no conflicts are executed immediately.
type Executor struct {
	metrics Metrics

	workers sync.WaitGroup

	outstanding sync.WaitGroup
	executable  chan *task

	nodes map[string]*task
	tasks int

	err uatomic.Error
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		nodes:      make(map[string]*task, items*2), // TODO: tune this
		executable: make(chan *task, items),         // ensure we don't block while holding lock
	}
	e.workers.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go e.work()
	}
	return e
}

func (e *Executor) work() {
	defer e.workers.Done()

	for {
		t, ok := <-e.executable
		if !ok {
			return
		}
		e.runTask(t)
	}
}

type blockingRequest struct {
	t     *task
	perms state.Permissions
}

type task struct {
	id      int
	f       func() error
	readers []*task

	l        sync.Mutex
	executed bool
	blocking map[int]*blockingRequest // next to be executed
	reading  map[int]*task            // using after execution

	dependencies atomic.Int64
}

func (e *Executor) runTask(t *task) {
	defer e.outstanding.Done()

	// We avoid doing this check when adding tasks to the queue
	// because it would require more synchronization.
	if e.err.Load() != nil {
		return
	}

	if err := t.f(); err != nil {
		e.err.CompareAndSwap(nil, err)
		return
	}

	// Clear readers
	for _, rt := range t.readers {
		rt.l.Lock()
		delete(rt.reading, t.id)
		rt.l.Unlock()
	}

	// Nodify blocking and mark as reading if needed
	t.l.Lock()
	for _, bt := range t.blocking {
		// Mark this task as reading, even if it isn't
		// executable yet.
		if bt.perms == state.Read {
			t.reading[bt.t.id] = bt.t
		}

		// If we are the last dependency, mark the task as executable.
		if bt.t.dependencies.Add(-1) > 0 {
			continue
		}
		e.executable <- bt.t
	}
	t.blocking = nil // free memory
	t.executed = true
	t.l.Unlock()
}

// Run executes [f] after all previously enqueued [f] with
// overlapping [keys] are executed.
//
// Run is not safe to call concurrently.
func (e *Executor) Run(keys state.Keys, f func() error) {
	e.outstanding.Add(1)

	// Add task to map
	id := e.tasks
	e.tasks++
	t := &task{
		id:      id,
		f:       f,
		readers: []*task{},

		blocking: make(map[int]*blockingRequest),
		reading:  make(map[int]*task),
	}

	// Add fake dependencies to ensure we don't execute the task
	// before we are finished enqueuing all dependencies.
	//
	// We can't have more than 1 dependency per key, so this effectively
	// ensures we don't get to 0 before the iteration is finished.
	dependenciesRemaining := int64(len(keys) + 1)
	t.dependencies.Add(dependenciesRemaining)

	// Record dependencies
	dependencies := set.NewSet[int](len(keys))
	for k, v := range keys {
		exclusive := v.Has(state.Allocate) || v.Has(state.Write)
		lt, ok := e.nodes[k]
		if ok {
			lt.l.Lock()

			// Handle the case where the dependency hasn't been executed yet
			if !lt.executed {
				// Provide the blocking task with enough
				// information to update its [reading] map
				// after it is executed.
				lt.blocking[id] = &blockingRequest{t, v}
				lt.l.Unlock()

				// Update the latest node if we neeed exclusive access
				// to the key.
				dependencies.Add(lt.id)
				if exclusive {
					e.nodes[k] = t
				} else {
					t.readers = append(t.readers, lt)
				}
				continue
			}

			// Handle the case where the dependency has already been executed
			if exclusive {
				for _, rt := range lt.reading {
					// TODO: May cause a deadlock here
					rt.l.Lock()
					rt.blocking[id] = &blockingRequest{t, v}
					rt.l.Unlock()
					dependencies.Add(rt.id)
				}
				e.nodes[k] = t
			} else {
				lt.reading[id] = t
				t.readers = append(t.readers, lt)
			}
			lt.l.Unlock()
		}
	}

	// Adjust dependency traker and execute if necessary
	extraDependencies := dependenciesRemaining - int64(dependencies.Len())
	if t.dependencies.Add(-extraDependencies) > 0 {
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
	e.err.CompareAndSwap(nil, ErrStopped)
}

// Wait returns as soon as all enqueued [f] are executed.
//
// You should not call [Run] after [Wait] is called.
func (e *Executor) Wait() error {
	e.outstanding.Wait()
	close(e.executable)
	e.workers.Wait()
	return e.err.Load()
}
