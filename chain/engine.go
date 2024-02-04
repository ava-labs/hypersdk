package chain

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
)

type engineJob struct {
	blk    *StatelessBlock
	chunks chan *Chunk
}

// Engine is in charge of orchestrating the execution of
// accepted chunks.
//
// TODO: put in VM?
type Engine struct {
	vm VM

	backlog chan *engineJob
}

func NewEngine(vm VM, maxBacklog int) *Engine {
	return &Engine{
		vm: vm,

		backlog: make(chan *engineJob, maxBacklog),
	}
}

func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case job := <-e.backlog:
			// if state is not height - 1, error
			p := NewProcessor(e.vm)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) Execute(blk *StatelessBlock) {
	// TODO: fetch chunks that don't exist (before start run) -> use a channel for the chunks so can start execution
	chunks := make(chan *Chunk, len(blk.AvailableChunks))

	// Enqueue job
	e.backlog <- &engineJob{
		blk:    blk,
		chunks: chunks,
	}
}

func (e *Engine) Results(height uint64) (ids.ID /* StartRoot */, []ids.ID /* Executed Chunks */, error) {
	// TODO: handle case where never started execution (state sync)
	return ids.ID{}, nil, errors.New("implement me")
}

func (e *Engine) Clear(height uint64) {
	// TODO: clear old tracking as soon as done
}
