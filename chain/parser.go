// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
)

type BlockParser struct {
	tracer trace.Tracer
	parser Parser
}

func NewBlockParser(
	tracer trace.Tracer,
	parser Parser,
) *BlockParser {
	return &BlockParser{
		tracer: tracer,
		parser: parser,
	}
}

func (b *BlockParser) ParseBlock(ctx context.Context, bytes []byte) (*ExecutionBlock, error) {
	_, span := b.tracer.Start(ctx, "Chain.ParseBlock")
	defer span.End()

	blk, err := UnmarshalBlock(bytes, b.parser)
	if err != nil {
		return nil, err
	}
	return NewExecutionBlock(blk), nil
}
