// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/state"
)

// Setup parent immutable state
var _ state.Immutable = &testDB{}

type testDB struct {
	storage map[string][]byte
}

func newTestDB() *testDB {
	return &testDB{
		storage: make(map[string][]byte),
	}
}

func newTestDBWithValue() *testDB {
	db := testDB{storage: make(map[string][]byte)}
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			db.storage[strconv.Itoa(i)] = []byte("value")
		}
	}
	return &db
}

func (db *testDB) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	val, ok := db.storage[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return val, nil
}

func TestFetchSameKeysConcurrentBasic(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(2, 4, newTestDB())
		ctx     = context.TODO()
	)
	var wg sync.WaitGroup
	wg.Add(2)

	sk := make(state.Keys, 1)
	sk.Add(strconv.Itoa(1), state.Read)

	txID1 := ids.GenerateTestID()
	txID2 := ids.GenerateTestID()

	wg1 := f.Lookup(ctx, txID1, sk)
	wg2 := f.Lookup(ctx, txID2, sk)

	go func(sk state.Keys, wg1 *sync.WaitGroup) {
		defer wg.Done()
		_, _ = f.Wait(wg1, sk)
	}(sk, wg1)

	go func(sk state.Keys, wg2 *sync.WaitGroup) {
		defer wg.Done()
		_, _ = f.Wait(wg2, sk)
	}(sk, wg2)

	wg.Wait()

	l := len(f.cache)
	require.Equal(1, l)
	require.Equal(2, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetchSameKeysConcurrent(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(10, 4, newTestDB())
		ctx     = context.TODO()
	)
	var wg sync.WaitGroup
	wg.Add(10)

	sk1 := make(state.Keys, 1)
	sk1.Add(strconv.Itoa(1), state.Read)

	sk2 := make(state.Keys, 2)
	sk2.Add(strconv.Itoa(1), state.Read)
	sk2.Add(strconv.Itoa(2), state.Read)

	sk3 := make(state.Keys, 1)
	sk1.Add(strconv.Itoa(3), state.Read)

	sk4 := make(state.Keys, 3)
	sk2.Add(strconv.Itoa(1), state.Read)
	sk2.Add(strconv.Itoa(2), state.Read)
	sk2.Add(strconv.Itoa(3), state.Read)

	for i := 0; i < 10; i++ {
		txID := ids.GenerateTestID()
		var fetchWg *sync.WaitGroup
		var sk state.Keys
		switch i % 4 {
		case 0:
			fetchWg = f.Lookup(ctx, txID, sk1)
			sk = sk1
		case 1:
			fetchWg = f.Lookup(ctx, txID, sk2)
			sk = sk2
		case 2:
			fetchWg = f.Lookup(ctx, txID, sk3)
			sk = sk3
		case 3:
			fetchWg = f.Lookup(ctx, txID, sk4)
			sk = sk4
		}
		go func(sk state.Keys, fwg *sync.WaitGroup) {
			defer wg.Done()
			_, _ = f.Wait(fwg, sk)
		}(sk, fetchWg)
	}

	wg.Wait()

	l := len(f.cache)
	require.Equal(3, l)
	require.Equal(10, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetchDifferentKeys(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(100, 4, newTestDB())
		ctx     = context.TODO()
	)
	for i := 0; i < 100; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate different read keys
			stateKeys.Add(ids.GenerateTestID().String(), state.Read)
		}
		txID := ids.GenerateTestID()
		// Since these are all different keys, we will
		// fetch each key from disk
		wg := f.Lookup(ctx, txID, stateKeys)
		_, _ = f.Wait(wg, stateKeys)
	}
	// There should be 5050 different keys now in the cache
	l := len(f.cache)
	require.Equal(5050, l)
	require.Equal(100, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetchSameKeys(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(100, 4, newTestDB())
		ctx     = context.TODO()
	)
	for i := 0; i < 100; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		// We are fetching the same keys, so we should
		// be getting subsequnt requests from cache
		wg := f.Lookup(ctx, txID, stateKeys)
		_, _ = f.Wait(wg, stateKeys)
	}
	// There's only 100 keys
	l := len(f.cache)
	require.Equal(100, l)
	require.Equal(100, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetchSameKeysSlow(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(25, 4, newTestDB())
		ctx     = context.TODO()
	)
	for i := 0; i < 25; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		wg := f.Lookup(ctx, txID, stateKeys)
		if i%2 == 0 {
			time.Sleep(1 * time.Second)
		}
		_, _ = f.Wait(wg, stateKeys)
	}
	l := len(f.cache)
	require.Equal(25, l)
	require.Equal(25, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetchKeysWithValues(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(100, 4, newTestDBWithValue())
		ctx     = context.TODO()
	)
	for i := 0; i < 100; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		wg := f.Lookup(ctx, txID, stateKeys)
		_, _ = f.Wait(wg, stateKeys)
	}
	l := len(f.cache)
	require.Equal(100, l)
	require.Equal(100, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetchSameKeyRepeatedly(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(100, 10, newTestDBWithValue())
		ctx     = context.TODO()
	)
	for i := 0; i < 100; i++ {
		stateKeys := make(state.Keys, 1)
		if i < 50 {
			stateKeys.Add(strconv.Itoa(1), state.Read)
		} else {
			stateKeys.Add(strconv.Itoa(2), state.Read)
		}
		txID := ids.GenerateTestID()
		wg := f.Lookup(ctx, txID, stateKeys)
		_, _ = f.Wait(wg, stateKeys)
	}
	l := len(f.cache)
	require.Equal(2, l)
	require.Equal(100, f.completed)
	require.NoError(f.HandleErrors())
}

func TestFetcherStop(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(100, 10, newTestDBWithValue())
		ctx     = context.TODO()
	)
	for i := 0; i < 100; i++ {
		stateKeys := make(state.Keys, 1)
		if i == 3 {
			f.Stop()
			break
		}
		txID := ids.GenerateTestID()
		wg := f.Lookup(ctx, txID, stateKeys)
		_, _ = f.Wait(wg, stateKeys)
	}
	require.Less(len(f.cache), 4)
	require.Equal(3, f.completed)
	require.Equal(ErrStopped, f.HandleErrors())
}
