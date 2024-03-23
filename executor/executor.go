// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"sync"

	"go.uber.org/atomic"

	"github.com/ava-labs/hypersdk/state"
)

// Executor sequences the concurrent execution of
// tasks with arbitrary conflicts on-the-fly.
//
// Executor ensures that conflicting tasks
// are executed in the order they were queued.
// Tasks with no conflicts are executed immediately.
type Executor struct {
	added int
	tasks []*task
	edges map[string]int

	outstanding sync.WaitGroup

	err atomic.Error
}

// New creates a new [Executor].
func New(items, _ int) *Executor {
	return &Executor{
		tasks: make([]*task, items),
		edges: make(map[string]int, items*2), // TODO: tune this
	}
}

type task struct {
	f func() error

	l        sync.Mutex
	waiters  map[int]*sync.WaitGroup
	executed bool
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
		e.err.CompareAndSwap(nil, errors.New("too many transactions created"))
		return
	}

	// Generate task
	id := e.added
	e.added++
	t := &task{
		f:       f,
		waiters: map[int]*sync.WaitGroup{},
	}
	e.tasks[id] = t
	e.outstanding.Add(1)

	// Record dependencies
	wg := sync.WaitGroup{}
	for k := range conflicts {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest]
			lt.l.Lock()
			if !lt.executed {
				wg.Add(1)
				lt.waiters[id] = &wg
			}
			lt.l.Unlock()
		}
		e.edges[k] = id
	}

	// Wait for the scheduler to execute us
	go func() {
		// Block until our dependencies have been executed
		wg.Wait()

		// Ensure we unblock our dependencies
		defer func() {
			t.l.Lock()
			for _, w := range t.waiters {
				w.Done()
			}
			t.waiters = nil
			t.executed = true
			t.l.Unlock()
			e.outstanding.Done()
		}()

		// Stop early if executor is stopped
		if e.err.Load() != nil {
			return
		}

		// Execute task once we aren't too busy
		if err := t.f(); err != nil {
			e.err.CompareAndSwap(nil, err)
			return
		}
	}()
}

func (e *Executor) Stop() {
	e.err.CompareAndSwap(nil, ErrStopped)
}

// Wait returns as soon as all enqueued [f] are executed.
//
// You should not call [Run] after [Wait] is called.
func (e *Executor) Wait() error {
	e.outstanding.Wait()
	return e.err.Load()
}
