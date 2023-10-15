package executor

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/set"
)

type Executor struct {
	wg      sync.WaitGroup
	workers []*worker
}

type worker struct {
	keys  map[string]int
	queue chan func()
}

func New(concurrency int) *Executor {
	e := &Executor{
		workers: make([]*worker, concurrency),
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
		e.workers[i] = &worker{
			keys:  map[string]int{},
			queue: ch,
		}
	}
	return e
}

func (e *Executor) Run(keys set.Set[string], f func()) {
}

func (e *Executor) Done() {
	for _, w := range e.workers {
		close(w.queue)
	}
	e.wg.Wait()
}
