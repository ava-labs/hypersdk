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

	tasks map[int]*task
	nodes map[string]*node

	err uatomic.Error
}

type node struct {
	id           int
	modification bool
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		tasks:      make(map[int]*task, items),
		nodes:      make(map[string]*node, items*2), // TODO: tune this
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

	for { //nolint:gosimple
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
	f func() error

	l        sync.Mutex
	blocking map[int]*task
	executed bool

	dependencies atomic.Int64

	allowConcurrentReading bool
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
		if bt.dependencies.Add(-1) > 0 {
			continue
		}
		bt.l.Lock()
		// We shouldn't be enqueuing concurrent Reads since they're not
		// dependent on each other
		if !bt.executed && !bt.allowConcurrentReading {
			bt.l.Unlock()
			e.executable <- bt
			bt.l.Lock()
		}
		bt.l.Unlock()
	}
	t.blocking = nil // free memory
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
	id := len(e.tasks)
	t := &task{
		f:        f,
		blocking: make(map[int]*task),
	}
	e.tasks[id] = t

	// Add dummy dependencies to ensure we don't execute the task
	dummyDependencies := int64(len(keys) + 1)
	t.dependencies.Add(dummyDependencies)

	// Record dependencies
	previousDependencies := set.NewSet[int](len(keys))
	hasConcurrentReads := false
	for k, v := range keys {
		n, ok := e.nodes[k]
		if ok {
			lt := e.tasks[n.id]
			lt.l.Lock()
			if !lt.executed {
				lt.blocking[id] = t

				switch {
				// concurrent Reads or Read(s)-after-Write
				case v == state.Read:
					// Reads aren't dependent on each other, so we
					// only consider Read(s)-after-Write case
					if n.modification {
						previousDependencies.Add(n.id)
					} else {
						hasConcurrentReads = true
					}
					lt.l.Unlock()
					continue
				case v.Has(state.Allocate) || v.Has(state.Write):
					// blocked by all Reads plus an Allocate/Write or the first Read
					// case 1: w->r->r...w->r->r, the length of [blocking] on the
					// second [w] is the number of reads from the first [w].
					// case 2: r->r->w..., the length of [blocking] is the number of
					// reads called before the first [w]
					for bid, bt := range lt.blocking {
						// this may happen in multi-key conflicts, where the id already
						// exists in blocking, so we don't want to record that we're
						// blocked on ourself
						if bid == id {
							continue
						}
						bt.l.Lock()
						if !bt.executed {
							previousDependencies.Add(bid) // may depend on the same task
							bt.blocking[id] = t
						}
						bt.l.Unlock()
					}
					// case 3: Write-after-Write
					previousDependencies.Add(n.id)
				}
			}
			lt.l.Unlock()
		}
		e.update(id, k, v)
	}

	// Adjust dependency traker and execute if necessary
	extraDependencies := dummyDependencies - int64(previousDependencies.Len())
	if t.dependencies.Add(-extraDependencies) > 0 {
		if e.metrics != nil {
			e.metrics.RecordBlocked()
		}
		return
	}

	if hasConcurrentReads {
		t.l.Lock()
		t.allowConcurrentReading = true
		t.l.Unlock()
	}
	// Mark task for execution if we aren't waiting on any other tasks
	e.executable <- t
	if e.metrics != nil {
		e.metrics.RecordExecutable()
	}
}

func (e *Executor) update(id int, k string, v state.Permissions) {
	e.nodes[k] = &node{id: id, modification: v.Has(state.Allocate) || v.Has(state.Write)}
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
