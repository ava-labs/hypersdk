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
	edges     map[string]*keyData
}

// keyData keeps track of the last blocked task, known
// as the id, its Permissions, and any tasks that
// are trying to read, but is blocked by the id,
// known as ConcurrentReads.
//
// ConcurrentReads is important in the case where we have
// many Reads that we want to access in parallel, followed
// by a Write. It isn't sufficient enough to note the id
// because we can't write if we're trying to read, and we can't
// read if we're trying to write. This is to adhere to the
// properties of the Executor.
//
// Consider the case where four transactions, touching the same state
// key, with the following permissions: T1_W -> T2_R -> T3_R -> T4_W.
// Assume that T1 hasn't executed, so this blocks T2 and T3. But,
// we also note down that T2 and T3 are concurrent reads here. So,
// when we get to T4, we observe that T2 and T3 are both blocking T4,
// and we can record the appropriate dependencies with ConcurrentReads.
type keyData struct {
	id              int
	permissions     state.Permissions
	concurrentReads set.Set[int]
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		stop:       make(chan struct{}),
		tasks:      make(map[int]*task, items),
		edges:      make(map[string]*keyData, items*2), // TODO: tune this
		executable: make(chan *task, items),            // ensure we don't block while holding lock
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
		// Get last blocked transaction
		key, exists := e.edges[k]
		if !exists {
			concurrentReads := set.NewSet[int](defaultSetSize)
			if v == state.Read {
				// Record any reads on the same key to help
				// with updating dependencies properly
				concurrentReads.Add(id)
			}
			// Key doesn't exist, so we add it to our edge map
			e.edges[k] = &keyData{id: id, permissions: v, concurrentReads: concurrentReads}
			continue
		}
		pt := e.tasks[key.id]
		if !pt.executed {
			if t.dependencies == nil {
				t.dependencies = set.NewSet[int](defaultSetSize)
			}
			if pt.blocking == nil {
				pt.blocking = set.NewSet[int](defaultSetSize)
			}
			switch {
			case v == state.Read:
				key.concurrentReads.Add(id)
				if key.permissions != state.Read {
					// Only record non-Read dependencies
					updateBlocking(pt, t, key.id)
				}
			case v.Has(state.Allocate) || v.Has(state.Write):
				if key.concurrentReads.Len() == 0 {
					// Blocked on write-after-writes
					updateBlocking(pt, t, key.id)
				} else {
					// Blocked when attempting write-after-reads
					// by all of the reads preceding the write
					e.updateConcurrentReads(key, t)
				}
				updateKey(key, id, v)
			}
			continue
		}
		// We don't record write-after-write, since the parent Write
		// has already executed. We consider any outstanding read(s)-
		// after-write that still need to be executed.
		if v.Has(state.Allocate) || v.Has(state.Write) {
			e.updateConcurrentReads(key, t)
		}
		if v == state.Read {
			key.concurrentReads.Clear()
			key.concurrentReads.Add(id)
		}
		updateKey(key, id, v)
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

// Occurs when trying to write-after-read or write-after-write
func updateBlocking(pt *task, t *task, keyID int) {
	pt.blocking.Add(t.id)
	t.dependencies.Add(keyID)
}

// Update id everytime it's a Allocate/Write key or if the last blocked txn ran
func updateKey(key *keyData, id int, v state.Permissions) {
	key.id = id
	key.permissions = v
}

// Update concurrentReads when we encounter a Allocate/Write key
// or when the last blocked transaction has been executed
func (e *Executor) updateConcurrentReads(key *keyData, t *task) {
	for b := range key.concurrentReads {
		bt := e.tasks[b]
		// Record that we're blocked on the concurrent reads
		// that are not yet executed.
		if !bt.executed {
			bt.blocking.Add(t.id)
			t.dependencies.Add(b)
		}
	}
	// Any read(s)-after-write will now be blocked by this task
	key.concurrentReads.Clear()
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
