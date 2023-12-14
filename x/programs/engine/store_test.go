// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// go test -v -benchmem -run=^$ -bench ^BenchmarkNewStore$ github.com/ava-labs/hypersdk/x/programs/engine -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkNewStore(b *testing.B) {
	eng := New(NewConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewStore(eng, NewStoreConfig())
	}
}

func TestMeteredStore(t *testing.T) {
	require := require.New(t)
	eng := New(NewConfig())
	// create a store
	maxUnits := uint64(100)
	store1 := NewStore(eng, NewStoreConfig())
	// add units to the store
	err := store1.AddUnits(maxUnits)
	require.NoError(err)
	// ensure balance is correct
	units, err := store1.GetBalanceUnits()
	require.NoError(err)
	require.Equal(maxUnits, units)
	// create a meter
	meter1, err := NewMeter(store1)
	require.NoError(err)
	// ensure balance is correct
	balance, err := meter1.GetBalance()
	require.NoError(err)
	require.Equal(maxUnits, balance)
	// add units to the store
	err = store1.AddUnits(maxUnits)
	require.NoError(err)
	// ensure balance is correct
	units, err = store1.GetBalanceUnits()
	require.NoError(err)
	require.Equal(maxUnits*2, units)
	// spend units from the store
	balance, err = store1.ConsumeUnits(maxUnits)
	require.NoError(err)
	require.Equal(maxUnits, balance)
	// ensure store can not be registered to two meters
	_, err = NewMeter(store1)
	require.ErrorIs(err, ErrMeteredStore)
	// create a new store with same engine
	store2 := NewStore(eng, NewStoreConfig())
	// ensure balance is correct
	units, err = store2.GetBalanceUnits()
	require.NoError(err)
	require.Equal(NoUnits, units)
	// create second meter
	meter2, err := NewMeter(store2)
	require.NoError(err)
	// transfer balance from meter 1 to meter 2
	balance, err = meter1.TransferUnitsTo(meter2, maxUnits)
	require.NoError(err)
	require.Equal(NoUnits, balance)
	// ensure balance is correct
	balance, err = meter2.GetBalance()
	require.NoError(err)
	require.Equal(maxUnits, balance)
	// attempt to overspend
	balance, err = meter2.Spend(maxUnits * 2)
	require.ErrorIs(err, ErrInsufficientUnits)
	// ensure balance is now zero
	require.Equal(NoUnits, balance)
}
