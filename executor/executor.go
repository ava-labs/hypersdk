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

	tasks           int
	maxDependencies int64
	nodes           map[string]*task

	err uatomic.Error
}

// New creates a new [Executor].
//
// It is assumed that no single task has more than [maxDependencies].
// This is used to ensure a dependent task does not begin executing a
// task until all dependencies have been enqueued. If this invariant is
// violated, some tasks will never execute and this code could deadlock.
func New(items, concurrency int, maxDependencies int64, metrics Metrics) *Executor {
	e := &Executor{
		metrics:         metrics,
		maxDependencies: maxDependencies,
		nodes:           make(map[string]*task, items*2), // TODO: tune this
		executable:      make(chan *task, items),         // ensure we don't block while holding lock
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

type task struct {
	id int
	f  func() error
	// reading are the tasks that this task is using non-exclusively.
	reading map[int]*task

	l        sync.Mutex
	executed bool
	// blocked are the tasks that are waiting on this task to execute
	// before they can be executed. tasks should not be added to
	// [blocked] after the task has been executed.
	blocked map[int]*task
	// readers are the tasks that only require non-exclusive access to
	// a key.
	readers map[int]*task

	// dependencies is synchronized outside of [l] so it can be adjusted while
	// we are setting up [task].
	dependencies atomic.Int64
}

func (e *Executor) runTask(t *task) {
	// No matter what happens, we need to clear our dependencies
	// to ensure we can exit.
	defer func() {
		// Notify other tasks that we are done reading them
		for _, rt := range t.reading {
			rt.l.Lock()
			delete(rt.readers, t.id)
			rt.l.Unlock()
		}
		t.reading = nil

		// Nodify blocked tasks that they can execute
		t.l.Lock()
		for _, bt := range t.blocked {
			// If we are the last dependency, mark the task as executable.
			if bt.dependencies.Add(-1) > 0 {
				continue
			}
			e.executable <- bt
		}
		t.blocked = nil // free memory
		t.executed = true
		t.l.Unlock()
		e.outstanding.Done()
	}()

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
}

// Run executes [f] after all previously enqueued [f] with
// overlapping [keys] are executed.
//
// Run is not safe to call concurrently.
//
// If there is an error, all remaining task execution will be skipped. It is up
// to the caller to ensure correctness does not depend on exactly when work begins
// to be skipped (i.e. not safe to rely on this functionality for block verification).
func (e *Executor) Run(keys state.Keys, f func() error) {
	e.outstanding.Add(1)

	// Add task to map
	id := e.tasks
	e.tasks++
	t := &task{
		id:      id,
		f:       f,
		reading: make(map[int]*task),

		blocked: make(map[int]*task),
		readers: make(map[int]*task),
	}

	// Add maximum number of dependencies to ensure we don't execute the task
	// before we are finished enqueuing all dependencies.
	//
	// We can have more than 1 dependency per key (in the case that there
	// are many readers for a single key), so we set this higher than we ever
	// expect to see.
	t.dependencies.Add(e.maxDependencies)

	// Record dependencies
	dependencies := set.NewSet[int](len(keys))
	for k, v := range keys {
		lt, ok := e.nodes[k]
		if ok {
			lt.l.Lock()
			if v == state.Read {
				// If we don't need exclusive access to a key, just mark
				// that we are reading it and that we are a reader of it.
				//
				// We use a map for [reading] because a single task can have
				// different keys that are all just readers of another task.
				t.reading[lt.id] = lt
				lt.readers[id] = t
			} else {
				// If we do need exclusive access to a key, we need to
				// mark ourselves blocked on all readers ahead of us.
				//
				// If a task is a reader, that means it is not executed yet
				// and can't mark itself as executed until all [reading] are
				// cleared (which can't be done while we hold the lock for [lt]).
				for _, rt := range lt.readers {
					// Don't block on ourself if we already marked ourself as a reader
					if rt.id == id {
						continue
					}
					rt.l.Lock()
					rt.blocked[id] = t
					rt.l.Unlock()
					dependencies.Add(rt.id)
				}
				e.nodes[k] = t
			}
			if !lt.executed {
				// If the task hasn't executed yet, we need to block on it.
				//
				// Note: this means that we first read as a block for subsequent reads. This
				// isn't the worst thing in the world because it prevents a cache stampede.
				lt.blocked[id] = t
				dependencies.Add(lt.id)
			}
			lt.l.Unlock()
			continue
		}
		e.nodes[k] = t
	}

	// Adjust dependency traker and execute if necessary
	difference := e.maxDependencies - int64(dependencies.Len())
	if t.dependencies.Add(-difference) > 0 {
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
