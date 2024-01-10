// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	//"context"
	//"fmt"
	"time"
	"testing"

	//"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/math"

	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"
)

func TestUnitPrice(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock VM
	rules := NewMockRules(ctrl)
	rules.EXPECT().GetMinUnitPrice().Return(Dimensions{100, 100, 100, 100, 100})

	// Similar to vm.go L326
	minUnitPrice := rules.GetMinUnitPrice()

	var d Dimensions
	feeManager := NewFeeManager(nil)
	for i := Dimension(0); i < FeeDimensions; i++ {
		// Set unit prices for different dimensions
		feeManager.SetUnitPrice(i, minUnitPrice[i])
		d[i] = uint64(i)
	}

	// Check unit prices
	unitPrices := feeManager.UnitPrices()
	for i := Dimension(0); i < FeeDimensions; i++ {
		require.Equal(unitPrices[i], feeManager.UnitPrice(i))
	}
}

func TestConsume(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	feeManager := NewFeeManager(nil)

	// Mock VM
	rules := NewMockRules(ctrl)
	// Similar to genesis
	rules.EXPECT().GetMaxBlockUnits().Return(Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000})
	rules.EXPECT().GetBaseComputeUnits().Return(uint64(1))
	rules.EXPECT().GetStorageKeyReadUnits().Return(uint64(5))
	rules.EXPECT().GetStorageValueReadUnits().Return(uint64(2))
	rules.EXPECT().GetStorageKeyAllocateUnits().Return(uint64(20))
	rules.EXPECT().GetStorageValueAllocateUnits().Return(uint64(5))
	rules.EXPECT().GetStorageKeyWriteUnits().Return(uint64(10))
	rules.EXPECT().GetStorageValueWriteUnits().Return(uint64(3))

	chunks := 100

	bandwidthOp := math.NewUint64Operator(200)
	bandwidthUnits, err := bandwidthOp.Value()
	require.NoError(err)

	computeUnitsOp := math.NewUint64Operator(rules.GetBaseComputeUnits())
	computeUnitsOp.Add(50)
	computeUnitsOp.Add(100)
	computeUnits, err := computeUnitsOp.Value()
	require.NoError(err)

	readsOp := math.NewUint64Operator(0)
	readsOp.Add(rules.GetStorageKeyReadUnits())
	readsOp.MulAdd(uint64(chunks), rules.GetStorageValueReadUnits())
	readUnits, err := readsOp.Value()
	require.NoError(err)

	allocatesOp := math.NewUint64Operator(0)
	allocatesOp.Add(rules.GetStorageKeyAllocateUnits())
	allocatesOp.MulAdd(uint64(chunks), rules.GetStorageValueAllocateUnits())
	allocateUnits, err := allocatesOp.Value()
	require.NoError(err)

	writesOp := math.NewUint64Operator(0)
	writesOp.Add(rules.GetStorageKeyWriteUnits())
	writesOp.MulAdd(uint64(chunks), rules.GetStorageValueWriteUnits())
	writeUnits, err := writesOp.Value()
	require.NoError(err)

	consumed := Dimensions{bandwidthUnits, computeUnits, readUnits, allocateUnits, writeUnits}

	// Similar to processor.go
	ok, dimension := feeManager.Consume(consumed, rules.GetMaxBlockUnits())
	require.True(ok)
	require.Equal(dimension, Dimension(0))

	// Fee required 
	_, err = feeManager.MaxFee(consumed)
	require.NoError(err)

	// Update last consumed
	for i := Dimension(0); i < FeeDimensions; i++ {
		feeManager.SetLastConsumed(i, consumed[i])
	}

	unitsConsumed := feeManager.UnitsConsumed()
	for i := Dimension(0); i < FeeDimensions; i++ {
		require.Equal(unitsConsumed[i], feeManager.LastConsumed(i))
	}	
}

func TestComputeNext(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Similar to genesis config
	rules := NewMockRules(ctrl)
	rules.EXPECT().GetWindowTargetUnits().Return(Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000})
	rules.EXPECT().GetUnitPriceChangeDenominator().Return(Dimensions{48, 48, 48, 48, 48})
	rules.EXPECT().GetMinUnitPrice().Return(Dimensions{100, 100, 100, 100, 100})

	// Create our parent "block"
	parentTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	nextTime := time.Now().UnixMilli()

	parentFeeManager := NewFeeManager(nil)
	_, err := parentFeeManager.ComputeNext(parentTime, nextTime, rules)
	require.NoError(err)
}