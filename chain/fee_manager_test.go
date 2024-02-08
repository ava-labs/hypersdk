// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"
	"testing"

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
	rules.EXPECT().GetMinUnitPrice().Return(Dimensions{100, 20, 70, 80, 200})

	// We set unit prices by the min unit price assigned at Genesis
	// This is similar to a usage described in:
	// https://github.com/ava-labs/hypersdk/blob/main/vm/vm.go#L326 
	minUnitPrice := rules.GetMinUnitPrice()
	lastConsumed := Dimensions{150, 30, 75, 90, 210}

	var d Dimensions
	feeManager := NewFeeManager(nil)
	for i := Dimension(0); i < FeeDimensions; i++ {
		// Set unit prices for different dimensions 
		feeManager.SetUnitPrice(i, minUnitPrice[i])
		// Set last consumed
		feeManager.SetLastConsumed(i, lastConsumed[i])
		d[i] = uint64(i)
	}

	// Check unit prices
	unitPrices := feeManager.UnitPrices()
	unitsConsumed := feeManager.UnitsConsumed()
	for i := Dimension(0); i < FeeDimensions; i++ {
		require.Equal(unitPrices[i], feeManager.UnitPrice(i))
		require.Equal(lastConsumed[i], unitsConsumed[i])
	}
}

func TestConsume(t *testing.T) {
	tests := []struct {
		name				string
		maxBlockUnits		Dimensions
		chunks				uint64
		expectedOk			bool
		expectedDim			Dimension
	}{
		{
			name: "all units are under max block units",
			maxBlockUnits: Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000},
			chunks: 100,
			expectedOk: true,
			expectedDim: Dimension(0),
		},
		{
			name: "read unit is over max block units",
			maxBlockUnits: Dimensions{2_000, 2_000, 2_000, 2_000, 2_000},
			chunks: 1000,
			expectedOk: false,
			expectedDim: Dimension(2),
		},		
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			feeManager := NewFeeManager(nil)

			// Mock VM
			rules := NewMockRules(ctrl)

			// Borrowed config values from 
			// https://github.com/ava-labs/hypersdk/blob/main/x/programs/cmd/simulator/vm/genesis/genesis.go 
			rules.EXPECT().GetMaxBlockUnits().Return(tt.maxBlockUnits)
			rules.EXPECT().GetBaseComputeUnits().Return(uint64(1))
			rules.EXPECT().GetStorageKeyReadUnits().Return(uint64(5))
			rules.EXPECT().GetStorageValueReadUnits().Return(uint64(2))
			rules.EXPECT().GetStorageKeyAllocateUnits().Return(uint64(20))
			rules.EXPECT().GetStorageValueAllocateUnits().Return(uint64(5))
			rules.EXPECT().GetStorageKeyWriteUnits().Return(uint64(10))
			rules.EXPECT().GetStorageValueWriteUnits().Return(uint64(3))

			// Get usage for bandwidth, compute, read, allocate, writes
			bandwidthOp := math.NewUint64Operator(200)
			bandwidthUnits, err := bandwidthOp.Value()
			require.NoError(err)

			computeUnitsOp := math.NewUint64Operator(rules.GetBaseComputeUnits())
			computeUnitsOp.Add(50)
			computeUnits, err := computeUnitsOp.Value()
			require.NoError(err)

			readsOp := math.NewUint64Operator(0)
			readsOp.Add(rules.GetStorageKeyReadUnits())
			readsOp.MulAdd(tt.chunks, rules.GetStorageValueReadUnits())
			readUnits, err := readsOp.Value()
			require.NoError(err)

			allocatesOp := math.NewUint64Operator(0)
			allocatesOp.Add(rules.GetStorageKeyAllocateUnits())
			allocatesOp.MulAdd(tt.chunks, rules.GetStorageValueAllocateUnits())
			allocateUnits, err := allocatesOp.Value()
			require.NoError(err)

			writesOp := math.NewUint64Operator(0)
			writesOp.Add(rules.GetStorageKeyWriteUnits())
			writesOp.MulAdd(tt.chunks, rules.GetStorageValueWriteUnits())
			writeUnits, err := writesOp.Value()
			require.NoError(err)

			consumed := Dimensions{bandwidthUnits, computeUnits, readUnits, allocateUnits, writeUnits}
			ok, dimension := feeManager.Consume(consumed, rules.GetMaxBlockUnits())
			require.Equal(tt.expectedOk, ok)
			require.Equal(tt.expectedDim, dimension)			
		})
	}
}

func TestComputeNext(t *testing.T) {
	tests := []struct {
		name			string
		parentTime		int64
		nextTime		int64
		targetUnits		Dimensions
		lastConsumed	Dimensions
		shouldErr		bool
	}{
		{
			name: "parent consumed units within rollup window",
			parentTime: time.Date(2024, time.January, 1, 1, 1, 0, 0, time.UTC).UnixMilli(),
			nextTime: time.Date(2024, time.January, 1, 1, 1, 5, 0, time.UTC).UnixMilli(),
			targetUnits: Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
			lastConsumed: Dimensions{100, 100, 100, 100, 100},
			shouldErr: false,
		},
		{
			name: "parent block used more units than its target",
			parentTime: time.Date(2024, time.January, 1, 1, 1, 0, 0, time.UTC).UnixMilli(),
			nextTime: time.Date(2024, time.January, 1, 1, 1, 5, 0, time.UTC).UnixMilli(),
			targetUnits: Dimensions{1, 1, 1, 1, 1},
			lastConsumed: Dimensions{100, 100, 100, 100, 100},
			shouldErr: false,
		},
		{
			name: "roll is greater than rollupWindow",
			parentTime: time.Date(2024, time.January, 1, 1, 1, 0, 0, time.UTC).UnixMilli(),
			nextTime: time.Date(2024, time.January, 1, 1, 2, 0, 0, time.UTC).UnixMilli(),
			targetUnits: Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
			lastConsumed: Dimensions{100, 100, 100, 100, 100},
			shouldErr: false,
		},		
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
		
			// Similar to genesis config
			rules := NewMockRules(ctrl)
			rules.EXPECT().GetWindowTargetUnits().Return(tt.targetUnits)
			rules.EXPECT().GetUnitPriceChangeDenominator().Return(Dimensions{48, 48, 48, 48, 48})
			rules.EXPECT().GetMinUnitPrice().Return(Dimensions{100, 100, 100, 100, 100})

			parentFeeManager := NewFeeManager(nil)

			for i := Dimension(0); i < FeeDimensions; i++ {
				parentFeeManager.SetLastConsumed(i, tt.lastConsumed[i])
			}	

			_, err := parentFeeManager.ComputeNext(tt.parentTime, tt.nextTime, rules)
			if tt.shouldErr {
				require.Error(err)
			} else {
				require.NoError(err)			
			}
		})
	}
}

func TestAdd(t *testing.T) {
	require := require.New(t)
	consumed := Dimensions{1,2,3,4,5}
	nconsumed := Dimensions{}
	tmpConsumed, err := Add(nconsumed, consumed)
	require.NoError(err)
	nconsumed = tmpConsumed
	for i := 0; i < FeeDimensions; i++ {
		require.Equal(nconsumed[i], consumed[i])
	}
}

func TestMaxFee(t *testing.T) {
	tests := []struct {
		name 				string
		unitPrices			Dimensions
		expected			uint64
	}{
		{
			name: "all unit prices are the same value",
			unitPrices: Dimensions{1, 1, 1, 1, 1},
			expected: uint64(10),
		},
		{
			name: "different unit prices",
			unitPrices: Dimensions{20, 50, 25, 52, 31},
			expected: uint64(380),
		},		
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			feeManager := NewFeeManager(nil)
			var d Dimensions
			for i := Dimension(0); i < FeeDimensions; i++ {
				// Set unit prices for different dimensions
				feeManager.SetUnitPrice(i, tt.unitPrices[i])
				d[i] = uint64(i)
			}

			// The fee required 
			feeRequired, err := feeManager.MaxFee(d)
			require.NoError(err)
			require.Equal(tt.expected, feeRequired)		
		})		
	}
}

func TestMulSum(t *testing.T) {
	tests := []struct {
		name 				string
		unitPrices			Dimensions
		maxUnits			Dimensions
		expected			uint64
	}{
		{
			name: "all units are the same value",
			unitPrices: Dimensions{1, 1, 1, 1, 1},
			maxUnits: Dimensions{1, 1, 1, 1, 1},
			expected: uint64(5),
		},
		{
			name: "different unit values",
			unitPrices: Dimensions{2, 3, 4, 5, 6},
			maxUnits: Dimensions{20, 50, 25, 52, 31},
			expected: uint64(736),
		},		
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			
			val, err := MulSum(tt.unitPrices, tt.maxUnits)
			require.NoError(err)
			require.Equal(tt.expected, val)
		})		
	}
}

func TestDimensionGreater(t *testing.T) {
	tests := []struct {
		name 			string
		d				Dimensions
		o				Dimensions
		expected		bool
	}{
		{
			name: "dimensions are greater",
			d: Dimensions{2,3,4,5,6},
			o: Dimensions{1,2,3,4,5},
			expected: true,
		},
		{
			name: "dimensions are less than",
			d: Dimensions{1,2,3,4,5},
			o: Dimensions{2,3,4,5,6},
			expected: false,
		},		
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			result := tt.d.Greater(tt.o)
			require.Equal(tt.expected, result)
		})
	}
}

func TestParseDimensions(t *testing.T) {
	tests := []struct {
		name			string
		input			[]string
		expected		Dimensions
		shouldErr		bool
	}{
		{
			name: "valid parse dimensions",
			input: []string{"0", "1", "2", "3", "4"},
			expected: Dimensions{0,1,2,3,4},
			shouldErr: false,
		},
		{
			name: "invalid number of dimensions",
			input: []string{"0", "1", "2", "3", "4", "5"},
			expected: Dimensions{},
			shouldErr: true,
		},	
		{
			name: "bad input",
			input: []string{"O", "!", "2", "3", "4"},
			expected: Dimensions{},
			shouldErr: true,
		},				
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			output, err := ParseDimensions(tt.input)
			if !tt.shouldErr {
				require.NoError(err)
				for i := 0; i < FeeDimensions; i++ {
					require.Equal(tt.expected[i], output[i])
				}				
			} else {
				require.Error(err)
			}
		})
	}
}