// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"fmt"
	"sync"
	"sync/atomic"

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

	tasks map[int]*task
	edges map[string]int

	err uatomic.Error
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		tasks:      make(map[int]*task, items),
		edges:      make(map[string]int, items*2), // TODO: tune this
		executable: make(chan *task, items),       // ensure we don't block while holding lock
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
		select {
		case t, ok := <-e.executable:
			if !ok {
				return
			}
			e.runTask(t)
		}
	}
}

type task struct {
	id int // TODO: remove
	f  func() error

	l        sync.Mutex
	blocking map[int]*task
	executed bool

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

	t.l.Lock()
	for _, bt := range t.blocking {
		deps := bt.dependencies.Add(-1)
		if deps > 0 {
			fmt.Println(t.id, "deps=", deps, "in task")
			continue
		}
		fmt.Println(t.id, "executing with deps=", deps, "in task")
		if deps != 0 {
			panic("deps should be 0")
		}
		e.executable <- bt
	}
	t.blocking = nil // free memory
	t.executed = true
	t.l.Unlock()
}

// Run executes [f] after all previously enqueued [f] with
// overlapping [conflicts] are executed.
//
// Run is not safe to call concurrently.
//
// TODO: Handle read-only/write-only keys (currently the executor
// treats everything still as ReadWrite, see issue below)
// https://github.com/ava-labs/hypersdk/issues/709
func (e *Executor) Run(conflicts state.Keys, f func() error) {
	e.outstanding.Add(1)

	// Generate task
	id := len(e.tasks)
	t := &task{
		id:       id,
		f:        f,
		blocking: map[int]*task{},
	}
	e.tasks[id] = t

	// Add dummy dependencies to ensure we don't execute the task
	dummyDependencies := int64(len(conflicts) + 1)
	t.dependencies.Add(dummyDependencies)

	// Record dependencies
	dependencies := 0
	for k := range conflicts {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest]
			lt.l.Lock()
			if !lt.executed {
				dependencies++
				fmt.Println(id, "adding dep", latest)
				lt.blocking[id] = t
			}
			lt.l.Unlock()
		}
		e.edges[k] = id
	}

	// Adjust dependency traker and execute if necessary
	extraDependencies := dummyDependencies - int64(dependencies)
	deps := t.dependencies.Add(-extraDependencies)
	if deps > 0 {
		fmt.Println(id, "deps=", deps, "at end of loop")
		if e.metrics != nil {
			e.metrics.RecordBlocked()
		}
		return
	}
	fmt.Println(id, "executing with deps=", deps, "at end of loop")
	if deps != 0 {
		panic("deps should be 0")
	}
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
