// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
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
	for i := 0; i < 100; i += 2 {
		db.storage[strconv.Itoa(i)] = []byte("value")
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

		l     sync.Mutex
		cache = set.Set[string]{}
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
		go func() {
			defer wg.Done()
			// Get keys from cache
			reads, storage, err := f.Get(txID)
			require.NoError(err)

			// Add to cache
			l.Lock()
			for k := range reads {
				cache.Add(k)
			}
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
		}()
	}
	wg.Wait()
	require.NoError(f.Wait())
	require.Len(cache, 5050)
}

func TestFetchSameKeys(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDB(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup

		l     sync.Mutex
		cache = set.Set[string]{}
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
		// be getting subsequent requests from cache
		require.NoError(f.Fetch(ctx, txID, stateKeys))
		go func() {
			defer wg.Done()
			reads, storage, err := f.Get(txID)
			require.NoError(err)
			l.Lock()
			for k := range reads {
				cache.Add(k)
			}
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
		}()
	}
	wg.Wait()
	require.NoError(f.Wait())
	require.Len(cache, 100)
}

func TestFetchSameKeysSlow(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 1000 // More txns trying to fetch same key
		f       = New(newTestDB(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup

		l     sync.Mutex
		cache = set.Set[string]{}
	)
	wg.Add(numTxs)
	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()

		// Empty chan to mimic timing out
		delay := make(chan struct{})

		// Fetch the key
		require.NoError(f.Fetch(ctx, txID, stateKeys))
		go func() {
			defer wg.Done()
			// Get the keys from cache
			reads, storage, err := f.Get(txID)
			require.NoError(err)

			// Notify we're done
			close(delay)
			l.Lock()
			for k := range reads {
				cache.Add(k)
			}
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
		}()
		// Wait a little to try to mimic the "stampede"
		<-delay
	}
	wg.Wait()
	require.NoError(f.Wait())
	require.Len(cache, 1000)
}

func TestFetchKeysWithValues(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDBWithValue(), numTxs, 4)
		ctx     = context.TODO()
		wg      sync.WaitGroup

		l     sync.Mutex
		cache = set.Set[string]{}
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
		go func() {
			defer wg.Done()
			reads, storage, err := f.Get(txID)
			require.NoError(err)
			l.Lock()
			for k := range reads {
				cache.Add(k)
			}
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
		}()
	}
	wg.Wait()
	require.NoError(f.Wait())
	require.Len(cache, 100)
}

func TestFetcherStop(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDBWithValue(), numTxs, 10)
		ctx     = context.TODO()
		wg      sync.WaitGroup

		l     sync.Mutex
		cache = set.Set[string]{}
	)
	wg.Add(numTxs)
	for i := 0; i < numTxs; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		err := f.Fetch(ctx, txID, stateKeys)
		if err != nil {
			// Some [Fetch] may return an error.
			// This happens after we called [Stop]
			require.Equal(ErrStopped, err)
		}
		go func(i int) {
			defer wg.Done()
			reads, storage, err := f.Get(txID)
			if err != nil {
				return
			}
			l.Lock()
			for k := range reads {
				cache.Add(k)
			}
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
			if i == 3 {
				f.Stop()
			}
		}(i)
	}
	wg.Wait()
	require.Equal(ErrStopped, f.Wait())
	require.Less(len(cache), 100)
}
