// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
)

type Accepter[T Action[T], A Auth[A]] struct {
	tracer         trace.Tracer
	validityWindow ValidityWindow[T, A]
	metrics        *chainMetrics
}

func NewAccepter[T Action[T], A Auth[A]](
	tracer trace.Tracer,
	validityWindow ValidityWindow[T, A],
	metrics *chainMetrics,
) *Accepter[T, A] {
	return &Accepter[T, A]{
		tracer:         tracer,
		validityWindow: validityWindow,
		metrics:        metrics,
	}
}

func (a *Accepter[T, A]) AcceptBlock(ctx context.Context, blk *OutputBlock[T, A]) error {
	_, span := a.tracer.Start(ctx, "Chain.AcceptBlock")
	defer span.End()

	a.metrics.txsAccepted.Add(float64(len(blk.Block.Txs)))
	a.validityWindow.Accept(blk)

	return blk.View.CommitToDB(ctx)
}
