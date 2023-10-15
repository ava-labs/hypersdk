package executor

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

type Executor struct {
	wg      sync.WaitGroup
	counts  map[string]*keyAssignment
	workers []chan func()
}

type keyAssignment struct {
	worker int
	count  int
}

func New(concurrency int) *Executor {
	e := &Executor{
		counts:  map[string]*keyAssignment{},
		workers: make([]chan func(), concurrency),
	}
	for i := 0; i < concurrency; i++ {
		e.wg.Add(1)
		ch := make(chan func())
		go func() {
			defer e.wg.Done()

			for f := range ch {
				f()
			}
		}()
		e.workers[i] = ch
	}
	return e
}

func (e *Executor) Run(keys set.Set[string], f func()) {
	// Find all conflicting jobs
	matches := []int{}
	for k := range keys {
		assignment, ok := e.counts[k]
		if !ok {
			continue
		}
		matches = append(matches, assignment.worker)
	}

	switch len(matches) {
	case 0:
		// We can add to any worker (prefer immediately executable)
	case 1:
		// We can enqueue behind a worker already bottlenecked
	default:
		// Required keys are being processed on different workers, we need to
		// merge execution
	}
}

func (e *Executor) Done() {
	for _, ch := range e.workers {
		close(ch)
	}
	e.wg.Wait()
}
