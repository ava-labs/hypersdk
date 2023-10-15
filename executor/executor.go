// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

const defaultSetSize = 8

// Executor sequences the concurrent execution of
// tasks with arbitrary conflicts on-the-fly.
//
// Executor ensures that conflicting tasks
// are executed in the order they were queued.
// Tasks with no conflicts are executed immediately.
type Executor struct {
	wg         sync.WaitGroup
	executable chan *task

	l         sync.Mutex
	done      bool
	completed int
	tasks     map[int]*task
	edges     map[string]int
}

// New creates a new [Executor].
func New(items, concurrency int) *Executor {
	e := &Executor{
		tasks:      map[int]*task{},
		edges:      map[string]int{},
		executable: make(chan *task, items), // ensure we don't block while holding lock
	}
	for i := 0; i < concurrency; i++ {
		e.createWorker()
	}
	return e
}

type task struct {
	id int
	f  func()

	dependencies set.Set[int]
	blocking     set.Set[int]

	executed bool
}

func (e *Executor) createWorker() {
	e.wg.Add(1)

	go func() {
		defer e.wg.Done()

		for t := range e.executable {
			t.f()

			e.l.Lock()
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
		}
	}()
}

// Run executes [f] after all previously enqueued [f] with
// overlapping [conflicts] are executed.
func (e *Executor) Run(conflicts set.Set[string], f func()) {
	e.l.Lock()
	defer e.l.Unlock()

	// Add task to map
	id := len(e.tasks)
	t := &task{
		id: id,
		f:  f,
	}
	e.tasks[id] = t

	// Record dependencies
	for k := range conflicts {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest]
			if !lt.executed {
				if t.dependencies == nil {
					t.dependencies = set.NewSet[int](defaultSetSize)
				}
				t.dependencies.Add(lt.id)
				if lt.blocking == nil {
					lt.blocking = set.NewSet[int](defaultSetSize)
				}
				lt.blocking.Add(id)
			}
		}
		e.edges[k] = id
	}

	// Start execution if there are no blocking dependencies
	if t.dependencies == nil || t.dependencies.Len() == 0 {
		t.dependencies = nil // free memory
		e.executable <- t
	}
}

// Wait returns as soon as all enqueued [f] are executed.
//
// You should not call [Run] after [Wait] is called.
func (e *Executor) Wait() {
	e.l.Lock()
	e.done = true
	if e.completed == len(e.tasks) {
		// We will close here if all tasks
		// are executed by the time we call [Wait].
		close(e.executable)
	}
	e.l.Unlock()
	e.wg.Wait()
}
