// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/math"
)

const (
	feeScaler = 0.8
)

func (vm *VM) SuggestedFee(ctx context.Context) (uint64, error) {
	ctx, span := vm.tracer.Start(ctx, "VM.SuggestedFee")
	defer span.End()

	last := vm.lastProcessed
	db, err := last.State()
	if err != nil {
		return 0, err
	}
	ubytes, err := db.GetValue(ctx, vm.StateManager().ParentUnitsConsumedKey())
	var u uint64
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return 0, err
	} else if err == nil {
		u = binary.BigEndian.Uint64(ubytes)
	}

	// We scale down unit price to prevent a spiral up in price
	r := vm.c.Rules(time.Now().Unix())
	return math.Max(
		uint64(float64(u)*feeScaler),
		r.GetMinUnitPrice(),
	), nil
}
