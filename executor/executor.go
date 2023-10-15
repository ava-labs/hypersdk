package executor

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

const defaultSetSize = 8

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

	// Add task to map
	id := len(e.tasks)
	t := &task{
		id: id,
		f:  f,
	}
	e.tasks[id] = t

	// Record dependencies
	for k := range keys {
		latest, ok := e.edges[k]
		if ok {
			lt := e.tasks[latest]
			if !lt.executed {
				if t.dependencies == nil {
					t.dependencies = set.NewSet[int](defaultSetSize)
				}
				t.dependencies.Add(lt.id)
				if lt.blocked == nil {
					lt.blocked = set.NewSet[int](defaultSetSize)
				}
				lt.blocked.Add(id)
			}
		}
		e.edges[k] = id
	}

	// Start execution if there are no blockers
	if t.dependencies == nil || t.dependencies.Len() == 0 {
		t.dependencies = nil // free memory
		e.executable <- t
	}
}

func (e *Executor) Done() {
	// close(e.executable) -> may not yet have everything sequenced
	e.wg.Wait()
}
