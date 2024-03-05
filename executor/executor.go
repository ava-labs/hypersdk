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
			// key has ONLY a Read permission
			if v == state.Read {
				key.concurrentReads.Add(id)
				// If the first read hasn't executed for some reason, and
				// we're doing a bunch of Reads with no conflicts like
				// R->R->R->..., we don't want to record any dependencies
				if key.permissions == state.Read {
					continue
				}
				// Last blocked transaction was a Allocate/Write
				pt.blocking.Add(id)
				t.dependencies.Add(key.id)
				continue
			}

			// key contains an Allocate/Write permission
			if v.Has(state.Allocate) || v.Has(state.Write) {
				// This may occur if we're doing a lot of W->W->W->...,
				// we still want to record that we're blocked
				if key.concurrentReads.Len() == 0 {
					pt.blocking.Add(id)
					t.dependencies.Add(key.id)
				} else {
					// With a bunch of reads before our write,
					// we need to update that we're blocked by
					// all of these reads
					e.updateConcurrentReads(key, t)
					// Any subsequent reads will now be blocked by this
					// task, if it hasn't executed yet.
					key.concurrentReads.Clear()
				}
			}
		} else {
			// We don't need to record the case of W->W->W->...,
			// since that parent write already executed. We
			// consider any outstanding reads that still need
			// to be executed once its parent write ran.
			if v.Has(state.Allocate) || v.Has(state.Write) {
				e.updateConcurrentReads(key, t)
			}
			key.concurrentReads.Clear()
			if v == state.Read {
				key.concurrentReads.Add(id)
			}
		}
		// Update id everytime it's a Allocate/Write key or if the parent ran
		key.id = id
		key.permissions = v
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
