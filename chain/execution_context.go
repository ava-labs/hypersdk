// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/window"
)

type ExecutionContext struct {
	NextUnitPrice  uint64
	NextUnitWindow window.Window
}

func GenerateExecutionContext(
	ctx context.Context,
	currTime int64, // ms
	parent *StatelessBlock,
	tracer trace.Tracer, //nolint:interfacer
	r Rules,
) (*ExecutionContext, error) {
	_, span := tracer.Start(ctx, "chain.GenerateExecutionContext")
	defer span.End()

	since := int((currTime - parent.Tmstmp) / consts.MillisecondsPerSecond) // convert to seconds
	nextUnitPrice, nextUnitWindow, err := computeNextPriceWindow(
		parent.UnitWindow,
		parent.UnitsConsumed,
		parent.UnitPrice,
		r.GetWindowTargetUnits(),
		r.GetUnitPriceChangeDenominator(),
		r.GetMinUnitPrice(),
		since,
	)
	if err != nil {
		return nil, err
	}
	return &ExecutionContext{
		NextUnitPrice:  nextUnitPrice,
		NextUnitWindow: nextUnitWindow,
	}, nil
}
