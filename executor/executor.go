// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"sync"
	"sync/atomic"
	_ "fmt"

	_ "github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/state"

	uatomic "go.uber.org/atomic"
)

// Executor sequences the concurrent execution of
// tasks with arbitrary keys on-the-fly.
//
// Executor ensures that conflicting tasks
// are executed in the order they were queued.
// Tasks with no keys are executed immediately.
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
	id int
	isAllocateWrite bool
}

// New creates a new [Executor].
func New(items, concurrency int, metrics Metrics) *Executor {
	e := &Executor{
		metrics:    metrics,
		tasks:      make(map[int]*task, items),
		nodes:      make(map[string]*node, items*2), // TODO: tune this
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
	id int
	f func() error

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

	//fmt.Printf("a sup %v\n", t.id)
	t.l.Lock()
	//fmt.Printf("b sup %v\n", t.id)
	for _, bt := range t.blocking {
		if bt.dependencies.Load() == 0 || bt.dependencies.Add(-1) > 0 {
			//fmt.Printf("hit %v %v %v\n", t.id, bt.id, len(e.executable))
			continue
		}
		//fmt.Printf("should never hit %v %v\n", t.id, bt.id)
		if !bt.executed {
			e.executable <- bt
		}
	}
	t.blocking = nil // free memory
	t.executed = true
	//fmt.Printf("c sup %v\n", t.id)
	t.l.Unlock()
}

// Run executes [f] after all previously enqueued [f] with
// overlapping [keys] are executed.
//
// Run is not safe to call concurrently.
func (e *Executor) Run(keys state.Keys, f func() error) {
	e.outstanding.Add(1)

	// Generate task
	id := len(e.tasks)
	t := &task{
		id: id,
		f:        f,
		blocking: map[int]*task{},
	}
	e.tasks[id] = t

	// Add dummy dependencies to ensure we don't execute the task
	//dummyDependencies := int64(len(keys) + 1)
	//t.dependencies.Add(dummyDependencies)

	// Record dependencies
	//previousDependencies := set.NewSet[int](len(keys))
	for k, v := range keys {
		n, ok := e.nodes[k]
		if ok {
			lt := e.tasks[n.id]
			lt.l.Lock()
			if !lt.executed {
				switch {
				case v == state.Read && !n.isAllocateWrite:
					lt.blocking[id] = t
					lt.l.Unlock()
					continue
				/*case v == state.Read && n.isAllocateWrite:
					t.dependencies.Add(int64(1))
					lt.blocking[id] = t
				case (v.Has(state.Allocate) || v.Has(state.Write)) && !n.isAllocateWrite:
					// blocked by all reads
					t.dependencies.Add(int64(len(lt.blocking)))*/
				case (v.Has(state.Allocate) || v.Has(state.Write)) && n.isAllocateWrite:
					t.dependencies.Add(int64(1))
					lt.blocking[id] = t					
				}
			}
			lt.l.Unlock()
		}
		e.nodes[k] = &node{id: id, isAllocateWrite: v.Has(state.Allocate) || v.Has(state.Write)}
	}

	// Adjust dependency traker and execute if necessary
	//extraDependencies := dummyDependencies - int64(previousDependencies.Len())
	//if t.dependencies.Add(-extraDependencies) > 0 {
	//fmt.Printf("dep %v\n", t.dependencies.Load())
	if t.dependencies.Load() > 0 {
		if e.metrics != nil {
			e.metrics.RecordBlocked()
		}
		return
	}

	//fmt.Printf("sending %v\n", id)
	// Mark task for execution if we aren't waiting on any other tasks
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