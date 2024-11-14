// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/hypersdk/internal/validity_window"
)

type Accepter struct {
	tracer         trace.Tracer
	validityWindow validity_window.TimeValidityWindow[*Transaction]
	metrics        *chainMetrics
}

func NewAccepter(
	tracer trace.Tracer,
	validityWindow validity_window.TimeValidityWindow[*Transaction],
	metrics *chainMetrics,
) *Accepter {
	return &Accepter{
		tracer:         tracer,
		validityWindow: validityWindow,
		metrics:        metrics,
	}
}

func (a *Accepter) AcceptBlock(ctx context.Context, blk *ExecutionBlock) error {
	_, span := a.tracer.Start(ctx, "Chain.AcceptBlock")
	defer span.End()

	a.metrics.txsAccepted.Add(float64(len(blk.StatelessBlock.Txs)))
	a.validityWindow.Accept(blk)

	return nil
}
