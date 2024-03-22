// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"sync"
	"sync/atomic"

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
	id int
	f  func() error

	l            sync.Mutex
	dependencies set.Set[int]
	blocking     set.Set[int]
	executed     bool
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
	for b := range t.blocking { // works fine on non-initialized map
		bt := e.tasks[b]
		bt.l.Lock() // BUG: what if need to update [t], but holding outer (a waiting for b and b waiting for a)?
		bt.dependencies.Remove(t.id)
		if bt.dependencies.Len() == 0 { // must be non-nil to be blocked
			bt.dependencies = nil // free memory
			e.executable <- bt
		}
		bt.l.Unlock()
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
		id: id,
		f:  f,
	}
	e.tasks[id] = t

	// Record dependencies
	t.l.Lock()
	defer t.l.Unlock()
	for k := range conflicts {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest]
			lt.l.Lock()
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
			lt.l.Unlock()
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
	abandonedTasks := len(e.tasks) - e.added
	if e.remaining.Add(-int64(abandonedTasks)) == 0 {
		e.stopOnce.Do(func() {
			close(e.stop)
		})
	}

	e.workers.Wait()
	return e.err
}
