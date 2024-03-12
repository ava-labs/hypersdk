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

func TestFetchDifferentKeys(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDB(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup
	)
	wg.Add(numTxs)

	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate different read keys
			stateKeys.Add(ids.GenerateTestID().String(), state.Read)
		}
		txID := ids.GenerateTestID()
		// Since these are all different keys, we will
		// fetch each key from disk
		require.NoError(f.Fetch(ctx, txID, stateKeys))
		go func(txID ids.ID) {
			defer wg.Done()
			_, _, err := f.Get(txID)
			require.NoError(err)
		}(txID)
	}
	wg.Wait()
	require.NoError(f.Wait())

	// There should be 5050 different keys now in the cache
	l := len(f.keys)
	require.Equal(5050, l)
}

func TestFetchSameKeys(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDB(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup
	)
	wg.Add(numTxs)

	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		// We are fetching the same keys, so we should
		// be getting subsequnt requests from cache
		require.NoError(f.Fetch(ctx, txID, stateKeys))
		go func(txID ids.ID) {
			defer wg.Done()
			_, _, err := f.Get(txID)
			require.NoError(err)
		}(txID)
	}
	wg.Wait()
	require.NoError(f.Wait())
	l := len(f.keys)
	require.Equal(numTxs, l)
}

func TestFetchSameKeysSlow(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 25
		f       = New(newTestDB(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup
	)
	wg.Add(numTxs)
	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		if i%2 == 0 {
			time.Sleep(1 * time.Second)
		}
		require.NoError(f.Fetch(ctx, txID, stateKeys))
		go func(txID ids.ID) {
			defer wg.Done()
			_, _, err := f.Get(txID)
			require.NoError(err)
		}(txID)
	}
	wg.Wait()
	require.NoError(f.Wait())
	l := len(f.keys)
	require.Equal(numTxs, l)
}

func TestFetchKeysWithValues(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDBWithValue(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup
	)
	wg.Add(numTxs)
	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		require.NoError(f.Fetch(ctx, txID, stateKeys))
		go func(txID ids.ID) {
			defer wg.Done()
			_, _, err := f.Get(txID)
			require.NoError(err)
		}(txID)
	}
	wg.Wait()
	require.NoError(f.Wait())
	l := len(f.keys)
	require.Equal(numTxs, l)
}

func TestFetcherStop(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDBWithValue(), numTxs, 10)
		ctx     = context.TODO()
		wg      sync.WaitGroup
	)
	wg.Add(numTxs)
	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		_ = f.Fetch(ctx, txID, stateKeys)
		go func(txID ids.ID, i int) {
			defer wg.Done()
			_, _, err := f.Get(txID)
			if err != nil {
				return
			}
			if i == 3 {
				f.Stop()
			}
		}(txID, i)
	}
	wg.Wait()
	require.Equal(ErrStopped, f.Wait())
	require.Less(len(f.keys), numTxs)
}
