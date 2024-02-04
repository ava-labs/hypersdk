package chain

import (
	"context"
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/state"
)

var ErrNotReady = errors.New("not ready")

type Executor struct {
	l        sync.Mutex
	complete bool
	err      error

	input  chan *Chunk
	output []ids.ID
}

func NewExecutor(chunks int) *Executor {
	return &Executor{
		input:  make(chan *Chunk, chunks),
		output: make([]ids.ID, 0, chunks),
	}
}

func (e *Executor) process(ctx context.Context, view state.View, chunk *Chunk) (ids.ID, error) {
	return ids.ID{}, errors.New("implement me")
}

func (e *Executor) Run(ctx context.Context, vctx VerifyContext) {
	view, err := vctx.View(ctx, true)
	if err != nil {
		e.l.Lock()
		e.complete = true
		e.err = ctx.Err()
		e.l.Unlock()
		return
	}

	for {
		select {
		case c, ok := <-e.input:
			if !ok {
				e.l.Lock()
				e.complete = true
				e.l.Unlock()
				return
			}

			e.l.Lock()
			if e.err != nil {
				e.l.Unlock()
				continue
			}

			filtered, err := e.process(ctx, view, c)
			e.l.Lock()
			if err != nil && e.err == nil {
				e.err = ctx.Err()
				e.l.Unlock()
				continue
			}
			e.output = append(e.output, filtered)
			e.l.Unlock()

		case <-ctx.Done():
			e.l.Lock()
			if e.err != nil {
				e.err = ctx.Err()
			}
			e.l.Unlock()
			return
		}
	}
}

func (e *Executor) Add(chunk *Chunk) {
	e.input <- chunk
}

func (e *Executor) Done() {
	close(e.input)
}

func (e *Executor) Results() ([]ids.ID, error) {
	e.l.Lock()
	defer e.l.Unlock()

	if !e.complete {
		return nil, ErrNotReady
	}
	if e.err != nil {
		return nil, e.err
	}
	return e.output, e.err
}
