// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	//"context"
	//"fmt"
	"time"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"
)

func TestSetUnitPrice(t *testing.T) {
	require := require.New(t)

	var d Dimensions
	feeManager := NewFeeManager(nil)
	for i := Dimension(0); i < FeeDimensions; i++ {
		// Set unit prices for different dimensions
		feeManager.SetUnitPrice(i, uint64(i))
		d[i] = uint64(i)
	}

	unitPrices := feeManager.UnitPrices()
	for i := Dimension(0); i < FeeDimensions; i++ {
		require.Equal(unitPrices[i], feeManager.UnitPrice(i))
	}

	_, err := feeManager.MaxFee(d)
	require.NoError(err)
}

func TestSetLastConsumed(t *testing.T) {
	require := require.New(t)
	feeManager := NewFeeManager(nil)
	for i := Dimension(0); i < FeeDimensions; i++ {
		feeManager.SetLastConsumed(i, uint64(i))
	}

	unitsConsumed := feeManager.UnitsConsumed()
	for i := Dimension(0); i < FeeDimensions; i++ {
		require.Equal(unitsConsumed[i], feeManager.LastConsumed(i))
	}
}

func TestConsume(t *testing.T) {
	require := require.New(t)
	feeManager := NewFeeManager(nil)

	consumed := Dimensions{100, 100, 100, 100, 100}
	maxUnits := Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000}

	ok, dimension := feeManager.Consume(consumed, maxUnits)
	require.True(ok)
	require.Equal(dimension, Dimension(0))
}

func TestComputeNext(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	controller := NewMockController(ctrl)
	rules := NewMockRules(ctrl)
	

	/*
	//var vm VM
	var r Rules
	parent := &StatelessBlock{
		StatefulBlock: &StatefulBlock{
			Prnt: ids.GenerateTestID(),
			Tmstmp: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
			Hght: 10000,
		},
	}

	parentTime := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	nextTime := time.Now().UnixMilli()
	//parentID := parent.ID()
	//r := parent.vm.Rules(nextTime)

	parentFeeManager := NewFeeManager(nil)
	_, err := parentFeeManager.ComputeNext(parentTime, nextTime, r)
	require.NoError(err)*/

	parent := NewGenesisBlock(ids.GenerateTestID())

}