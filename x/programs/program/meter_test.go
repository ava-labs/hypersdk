// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"testing"


	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
)

func TestMeter(t *testing.T) {
	require := require.New(t)
	eng := engine.New(engine.NewConfig())
	store := engine.NewStore(eng, engine.NewStoreConfig(1))

	// new meter balance is 0
	meter, err := NewMeter(store, 0)
	require.NoError(err)
	// increase balance
	units := uint64(10000)
	balance, err := meter.AddUnits(units)
	require.NoError(err)
	require.Equal(units, balance)
	require.Equal(units, meter.GetBalance())
	// verify over spend fails
	_, err = meter.Spend(units +1)
	require.ErrorIs(err, ErrInsufficientUnits)
	// overspend should consume all units
	balance = meter.GetBalance()
	require.Equal(uint64(0), balance)
}

func TestMeterMultipleStores(t *testing.T) {
	require := require.New(t)
	eng := engine.New(engine.NewConfig())
	store1 := engine.NewStore(eng, engine.NewStoreConfig(1))
	store2 := engine.NewStore(eng, engine.NewStoreConfig(1))

	// ensure isolation between stores in same engine.
	units := uint64(100)
	meter1, err := NewMeter(store1, units)
	require.NoError(err)
	meter2, err := NewMeter(store2, 0)
	require.NoError(err)
	require.Equal(units, meter1.GetBalance())
	require.Equal(uint64(0), meter2.GetBalance())
	// transfer units from meter1 to meter2
	balance, err := meter1.TransferUnitsTo(meter2, units)
	require.NoError(err)
	require.Equal(uint64(0), balance)
	// transfer too many units from meter2 to meter1
	balance, err = meter2.TransferUnitsTo(meter1, units + 1)
	require.ErrorIs(err, ErrInsufficientUnits)
	require.Equal(uint64(0), balance)
	require.Equal(uint64(0), meter2.GetBalance())
	require.Equal(uint64(0), meter1.GetBalance())
}