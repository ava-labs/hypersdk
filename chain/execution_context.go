// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/window"
)

type ExecutionContext struct {
	ChainID ids.ID

	NextUnitPrice  uint64
	NextUnitWindow window.Window

	NextBlockCost   uint64
	NextBlockWindow window.Window
}

func computeNextPriceWindow(
	previous window.Window,
	previousConsumed uint64,
	previousPrice uint64,
	target uint64, /* per window */
	changeDenom uint64,
	minPrice uint64,
	since int, /* seconds */
) (uint64, window.Window, error) {
	newRollupWindow, err := window.Roll(previous, since)
	if err != nil {
		return 0, window.Window{}, err
	}
	if since < window.WindowSize {
		// add in the units used by the parent block in the correct place
		// If the parent consumed units within the rollup window, add the consumed
		// units in.
		slot := window.WindowSize - 1 - since
		start := slot * consts.Uint64Len
		window.Update(&newRollupWindow, start, previousConsumed)
	}
	total := window.Sum(newRollupWindow)

	nextPrice := previousPrice
	if total > target {
		// If the parent block used more units than its target, the baseFee should increase.
		delta := total - target
		x := previousPrice * delta
		y := x / target
		baseDelta := y / changeDenom
		if baseDelta < 1 {
			baseDelta = 1
		}
		n, over := math.Add64(nextPrice, baseDelta)
		if over != nil {
			nextPrice = consts.MaxUint64
		} else {
			nextPrice = n
		}
	} else if total < target {
		// Otherwise if the parent block used less units than its target, the baseFee should decrease.
		delta := target - total
		x := previousPrice * delta
		y := x / target
		baseDelta := y / changeDenom
		if baseDelta < 1 {
			baseDelta = 1
		}

		// If [roll] is greater than [rollupWindow], apply the state transition to the base fee to account
		// for the interval during which no blocks were produced.
		// We use roll/rollupWindow, so that the transition is applied for every [rollupWindow] seconds
		// that has elapsed between the parent and this block.
		if since > window.WindowSize {
			// Note: roll/rollupWindow must be greater than 1 since we've checked that roll > rollupWindow
			baseDelta *= uint64(since / window.WindowSize)
		}
		n, under := math.Sub(nextPrice, baseDelta)
		if under != nil {
			nextPrice = 0
		} else {
			nextPrice = n
		}
	}
	if nextPrice < minPrice {
		nextPrice = minPrice
	}
	return nextPrice, newRollupWindow, nil
}

func GenerateExecutionContext(
	ctx context.Context,
	chainID ids.ID,
	currTime int64,
	parent *StatelessBlock,
	tracer trace.Tracer, //nolint:interfacer
	r Rules,
) (*ExecutionContext, error) {
	_, span := tracer.Start(ctx, "chain.GenerateExecutionContext")
	defer span.End()

	since := int(currTime - parent.Tmstmp)
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
	nextBlockCost, nextBlockWindow, err := computeNextPriceWindow(
		parent.BlockWindow,
		1,
		parent.BlockCost,
		r.GetWindowTargetBlocks(),
		r.GetBlockCostChangeDenominator(),
		r.GetMinBlockCost(),
		since,
	)
	if err != nil {
		return nil, err
	}
	return &ExecutionContext{
		ChainID: chainID,

		NextUnitPrice:  nextUnitPrice,
		NextUnitWindow: nextUnitWindow,

		NextBlockCost:   nextBlockCost,
		NextBlockWindow: nextBlockWindow,
	}, nil
}
