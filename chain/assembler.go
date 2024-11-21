// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/state"
)

type Assembler struct {
	tracer    trace.Tracer
	processor *Processor
}

func NewAssembler(tracer trace.Tracer, processor *Processor) *Assembler {
	return &Assembler{
		tracer:    tracer,
		processor: processor,
	}
}

func (a *Assembler) AssembleBlock(
	ctx context.Context,
	parentView state.View,
	parent *ExecutionBlock,
	timestamp int64,
	blockHeight uint64,
	txs []*Transaction,
) (*ExecutedBlock, state.View, error) {
	ctx, span := a.tracer.Start(ctx, "Chain.AssembleBlock")
	defer span.End()

	parentStateRoot, err := parentView.GetMerkleRoot(ctx)
	if err != nil {
		return nil, nil, err
	}

	sb, err := NewStatelessBlock(
		parent.ID(),
		timestamp,
		blockHeight,
		txs,
		parentStateRoot,
	)
	if err != nil {
		return nil, nil, err
	}
	executionBlock, err := NewExecutionBlock(sb)
	if err != nil {
		return nil, nil, err
	}
	return a.processor.Execute(ctx, parentView, executionBlock)
}
