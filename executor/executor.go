// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ava-labs/hypersdk/state"
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

	executable chan *task
	remaining  atomic.Int64

	added int
	tasks []*task
	edges map[string]int

	stop     chan struct{}
	err      error
	stopOnce sync.Once
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		tasks:      make([]*task, items),
		edges:      make(map[string]int, items*2), // TODO: tune this
		executable: make(chan *task, items),       // ensure we don't block while holding lock
		stop:       make(chan struct{}),
	}
	e.remaining.Add(int64(items))
	e.workers.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go e.createWorker()
	}
	return e
}

type task struct {
	f func() error

	l        sync.Mutex
	blocking map[int]*task
	executed bool

	dependencies atomic.Int64
}

func (e *Executor) runTask(t *task) {
	defer func() {
		if e.remaining.Add(-1) == 0 {
			e.stopOnce.Do(func() {
				close(e.stop)
			})
		}
	}()

	if err := t.f(); err != nil {
		e.stopOnce.Do(func() {
			e.err = err
			close(e.stop)
		})
		return
	}

	t.l.Lock()
	for _, bt := range t.blocking { // works fine on non-initialized map
		if bt.dependencies.Add(-1) == 0 {
			e.executable <- bt
		}
	}
	t.blocking = nil // free memory
	t.executed = true
	t.l.Unlock()
}

func (e *Executor) createWorker() {
	defer e.workers.Done()

	for {
		select {
		case t := <-e.executable:
			e.runTask(t)
		case <-e.stop:
			return
		}
	}
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
	// Ensure too many transactions not enqueued
	if e.added >= len(e.tasks) {
		e.stopOnce.Do(func() {
			e.err = errors.New("too many transactions created")
			close(e.stop)
		})
		return
	}

	// Generate task
	id := e.added
	e.added++
	t := &task{
		f:        f,
		blocking: map[int]*task{},
	}
	e.tasks[id] = t

	// Record dependencies
	dummyDependencies := int64(len(conflicts) + 1)
	t.dependencies.Add(dummyDependencies) // ensure there is no way we can be executed until we have registered all dependencies
	for k := range conflicts {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest]
			lt.l.Lock()
			if !lt.executed {
				t.dependencies.Add(1)
				lt.blocking[id] = t
			}
			lt.l.Unlock()
		}
		e.edges[k] = id
	}

	// Start execution if there are no blocking dependencies
	if t.dependencies.Add(-dummyDependencies) == 0 {
		e.executable <- t
		e.metrics.RecordExecutable()
		return
	}
	e.metrics.RecordBlocked()
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
	abandonedTasks := len(e.tasks) - e.added
	if e.remaining.Add(-int64(abandonedTasks)) == 0 {
		e.stopOnce.Do(func() {
			close(e.stop)
		})
	}

	e.workers.Wait()
	return e.err
}
