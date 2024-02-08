// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"

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
	edges     map[string]*KeyData
	blocking  set.Set[int]
}

// KeyData keeps track of the key permission and the
// TaskID using the state key name.
type KeyData struct {
	TaskID      int
	Permissions state.Permissions
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		stop:       make(chan struct{}),
		tasks:      make(map[int]*task, items),
		edges:      make(map[string]*KeyData, items*2), // TODO: tune this
		executable: make(chan *task, items),            // ensure we don't block while holding lock
		blocking:   set.NewSet[int](defaultSetSize),    // txns that are executing that are blocking other txns
	}
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
				e.l.Lock()
				for b := range t.blocking { // works fine on non-initialized map
					bt := e.tasks[b]
					bt.dependencies.Remove(t.id)
					e.blocking.Remove(t.id)         // if a txn was blocking other txn, we can move remove it
					if bt.dependencies.Len() == 0 { // must be non-nil to be blocked
						bt.dependencies = nil // free memory
						e.executable <- bt
					}
				}
				t.blocking = nil // free memory
				t.executed = true
				e.completed++
				if e.done && e.completed == len(e.tasks) {
					e.blocking = nil // free memory
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

// Run executes [f] after all previously enqueued [f] with
// overlapping [conflicts] are executed.
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
	for k, v := range conflicts {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest.TaskID]
			if !lt.executed {
				if t.dependencies == nil {
					t.dependencies = set.NewSet[int](defaultSetSize)
				}
				if lt.blocking == nil {
					lt.blocking = set.NewSet[int](defaultSetSize)
				}
				// key has ONLY a Read permission
				if v == state.Read {
					t.dependencies.Add(lt.id) // t depends on lt to execute
					lt.blocking.Add(id)       // lt is blocking this current task
					// If lt is blocking any other txn, we need to record this
					// to prevent subsequent txns that depend on lt from executing
					// at the same time
					e.blocking.Add(lt.id)
					continue
				}

				// key contains a Allocate or Write permission
				if v.Has(state.Allocate) || v.Has(state.Write) {
					// lt contains either Read, Allocate, or Write
					if latest.Permissions.Has(state.Read) || latest.Permissions.Has(state.Allocate) || latest.Permissions.Has(state.Write) {
						t.dependencies.Add(lt.id)
						lt.blocking.Add(id)
						e.blocking.Add(lt.id)
					}
				}
			}
		}
		// key doesn't exist in our edges map, so we add it
		e.edges[k] = &KeyData{TaskID: id, Permissions: v}
	}

	if t.dependencies != nil && t.dependencies.Len() > 0 {
		// Ensure that we don't schedule a transaction that is
		// blocked by another transaction currently running
		for bID := range t.dependencies {
			// don't execute if any dependency task is in blocking
			if e.blocking.Contains(bID) {
				if e.metrics != nil {
					e.metrics.RecordBlocked()
				}
				return
			}
		}
	}
	// Start execution if there are no blocking dependencies
	t.dependencies = nil // free memory
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
		e.blocking = nil // free memory
		// We will close here if all tasks
		// are executed by the time we call [Wait].
		close(e.executable)
	}
	e.l.Unlock()
	e.wg.Wait()
	return e.err
}
