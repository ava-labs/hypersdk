// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
)

type BlockParser[T Action[T], A Auth[A]] struct {
	tracer trace.Tracer
}

func NewBlockParser[T Action[T], A Auth[A]](
	tracer trace.Tracer,
) *BlockParser[T, A] {
	return &BlockParser[T, A]{
		tracer: tracer,
	}
}

func (b *BlockParser[T, A]) ParseBlock(ctx context.Context, bytes []byte) (*ExecutionBlock[T, A], error) {
	_, span := b.tracer.Start(ctx, "Chain.ParseBlock")
	defer span.End()

	blk, err := ParseBlock[T, A](bytes)
	if err != nil {
		return nil, err
	}
	return NewExecutionBlock(blk), nil
}
