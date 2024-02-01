// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/state"
)

const defaultSetSize = 8

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
	edges     map[string]int
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		stop:       make(chan struct{}),
		tasks:      make(map[int]*task, items),
		edges:      make(map[string]int, items*2), // TODO: tune this
		executable: make(chan *task, items),       // ensure we don't block while holding lock
	}
	fmt.Printf("BBBBB items %v\n", items)
	for i := 0; i < concurrency; i++ {
		e.createWorker()
	}
	return e
}

type task struct {
	id int
	f  func() error

	dependencies set.Set[int]
	blocking     set.Set[int]

	executed bool
}

func (e *Executor) createWorker() {
	e.wg.Add(1)

	go func() {
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
				fmt.Printf("its running ID %v\n", t.id)
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
			case <-e.stop:
				return
			}
		}
	}()
}

/*
t.id = 1 | e.edges: {a: 1} | t.dep: () | t.block: (2)
	bt = Task2
		Task2 dep, remove that Task2 is dep on Task1
		If no more dep then I can execute Task2 and it gets put in the e.executable chan queue

t.id = 2 | e.edges: {a: 1} | t.dep: (1) | t.block: ()
	bt = Task1

	...After getting its dep removed
		t.dep is empty
	...After waiting in the e.executable chan queue
	This task doesn't block anything so it just executes
*/

// Run executes [f] after all previously enqueued [f] with
// overlapping [conflicts] are executed.
// TODO: Handle read-only/write-only keys (currently the executor
// treats everything still as ReadWrite, see issue below)
// https://github.com/ava-labs/hypersdk/issues/709
func (e *Executor) Run(conflicts state.Keys, f func() error) {
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
		fmt.Printf("AAAAA in edge set %v\n", len(e.edges))
		if ok {
			lt := e.tasks[latest]
			fmt.Printf("record dep ID %v\n", lt.id)
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
		if e.metrics != nil {
			e.metrics.RecordExecutable()
		}
		return
	}
	if e.metrics != nil {
		e.metrics.RecordBlocked()
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
