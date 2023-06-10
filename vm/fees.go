// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/AnomalyFi/hypersdk/chain"
)

const (
	feeScaler = 0.8
)

func (vm *VM) SuggestedFee(ctx context.Context) (uint64, uint64, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.SuggestedFee")
	defer span.End()

	rpreferred, err := vm.GetBlock(ctx, vm.preferred)
	if err != nil {
		return 0, 0, err
	}
	preferred := rpreferred.(*chain.StatelessBlock)

	// We scale down unit price to prevent a spiral up in price
	r := vm.c.Rules(time.Now().Unix())
	return math.Max(
			uint64(float64(preferred.UnitPrice)*feeScaler),
			r.GetMinUnitPrice(),
		),
		math.Max(
			uint64(float64(preferred.BlockCost)*feeScaler),
			r.GetMinBlockCost(),
		), nil
}
