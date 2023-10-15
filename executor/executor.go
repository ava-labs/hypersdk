package executor

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

type Executor struct {
	wg sync.WaitGroup

	l          sync.Mutex
	tasks      map[int]*task
	edges      map[string]int
	executable chan *task
}

type task struct {
	id int
	f  func()

	dependencies set.Set[int]
	blocked      set.Set[int]

	executed bool
}

func (e *Executor) createWorker() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()

		for t := range e.executable {
			t.f()

			e.l.Lock()
			for b := range t.blocked {
				bt := e.tasks[b]
				bt.dependencies.Remove(t.id)
				if bt.dependencies.Len() == 0 {
					bt.dependencies = nil // free memory
					e.executable <- bt
				}
			}
			t.blocked = nil // free memory
			t.executed = true
			e.l.Unlock()
		}
	}()
}

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

// Run ensures that any [f] with dependencies is executed in order.
func (e *Executor) Run(keys set.Set[string], f func()) {
	e.l.Lock()
	defer e.l.Unlock()

	// Find all conflicting jobs
	matches := []int{}
	for k := range keys {
		assignment, ok := e.counts[k]
		if !ok {
			continue
		}
		matches = append(matches, assignment.worker)
	}

	// Schedule function on some worker
	switch len(matches) {
	case 0:
		// We can add to any worker (prefer immediately executable)
	case 1:
		// We can enqueue behind a worker already bottlenecked
		e.workers[matches[0]] <- f
	default:
		// Required keys are being processed on different workers, we need to
		// merge execution after waiting for dependencies to finish
	}
}

func (e *Executor) Done() {
	for _, ch := range e.workers {
		close(ch)
	}
	e.wg.Wait()
}
