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

const baseDependencies = 1_000_000

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

type requester struct {
	t     *task
	perms state.Permissions
}

type task struct {
	id      int
	f       func() error
	reading []*task

	l          sync.Mutex
	executed   bool
	requesters map[int]*requester
	readers    map[int]*task

	dependencies atomic.Int64
}

func (e *Executor) runTask(t *task) {
	defer e.outstanding.Done()

	// We avoid doing this check when adding tasks to the queue
	// because it would require more synchronization.
	if e.err.Load() != nil {
		return
	}

	// Execute the task
	if err := t.f(); err != nil {
		e.err.CompareAndSwap(nil, err)
		return
	}

	// Notify other tasks that we are done reading
	// them (if we only require non-exclusive access)
	for _, rt := range t.reading {
		rt.l.Lock()
		delete(rt.readers, t.id)
		rt.l.Unlock()
	}
	t.reading = nil

	// Nodify requesters that they can execute
	t.l.Lock()
	for _, bt := range t.requesters {
		// As soon as we clear a requester, make sure we mark
		// that they are now reading us.
		if bt.perms == state.Read {
			t.reading[bt.t.id] = bt.t
		}

		// If we are the last dependency, mark the task as executable.
		if bt.t.dependencies.Add(-1) > 0 {
			continue
		}
		e.executable <- bt.t
	}
	t.requesters = nil // free memory
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
		reading: []*task{},

		requesters: make(map[int]*requester),
		readers:    make(map[int]*task),
	}

	// Add fake dependencies to ensure we don't execute the task
	// before we are finished enqueuing all dependencies.
	//
	// We can have more than 1 dependency per key (in the case that there
	// are many readers for a single key), so we set this higher than we ever
	// expect to see.
	t.dependencies.Add(baseDependencies)

	// Record dependencies
	dependencies := set.NewSet[int](len(keys))
	for k, v := range keys {
		exclusive := v.Has(state.Allocate) || v.Has(state.Write)
		lt, ok := e.nodes[k]
		if ok {
			// If we don't need exclusive access to a key, make sure
			// to mark that we are reading it.
			if !exclusive {
				t.reading = append(t.reading, lt)
			}

			// Handle the case where the dependency hasn't been executed yet
			lt.l.Lock()
			if !lt.executed {
				lt.requesters[id] = &requester{t, v}
				lt.l.Unlock()
				dependencies.Add(lt.id)

				// If we need exclusive access to the node, we update
				// [nodes] to ensure anyone else that needs access to the key
				// after us can only do so after we are done.
				if exclusive {
					e.nodes[k] = t
				}
				continue
			}

			// Handle the case where we need to read something that has already been executed
			if !exclusive {
				lt.readers[id] = t
				lt.l.Unlock()
				continue
			}

			// Handle the case where the dependency has already been executed and we need exclusive access
			//
			// We are guaranteed that any [reader] has not been executed yet, so we can safely put a dependency
			// on these readers.
			for _, rt := range lt.readers {
				rt.l.Lock()
				rt.requesters[id] = &requester{t, v}
				rt.l.Unlock()
				dependencies.Add(rt.id)
			}
			e.nodes[k] = t
			lt.l.Unlock()
			continue
		}
		e.nodes[k] = t
	}

	// Adjust dependency traker and execute if necessary
	extraDependencies := baseDependencies - int64(dependencies.Len())
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
