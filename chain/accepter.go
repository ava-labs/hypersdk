// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
)

type Accepter struct {
	tracer         trace.Tracer
	validityWindow ValidityWindow
	metrics        *ChainMetrics
}

func NewAccepter(
	tracer trace.Tracer,
	validityWindow ValidityWindow,
	metrics *ChainMetrics,
) *Accepter {
	return &Accepter{
		tracer:         tracer,
		validityWindow: validityWindow,
		metrics:        metrics,
	}
}

func (a *Accepter) AcceptBlock(ctx context.Context, blk *OutputBlock) error {
	_, span := a.tracer.Start(ctx, "Chain.AcceptBlock")
	defer span.End()

	a.metrics.txsAccepted.Add(float64(len(blk.StatelessBlock.Txs)))
	a.validityWindow.Accept(blk)

	return blk.View.CommitToDB(ctx)
}
