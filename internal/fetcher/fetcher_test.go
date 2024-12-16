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

const keyBase = "blah" // key must be long enough to be valid

// Setup parent immutable state
var _ state.Immutable = (*testDB)(nil)

type testDB struct {
	storage map[string][]byte
}

func newTestDB() *testDB {
	db := testDB{storage: make(map[string][]byte)}
	for i := 0; i < 100; i += 2 {
		db.storage[keyBase+strconv.Itoa(i)] = []byte("value") // key must be long enough to be valid
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
		keys := make([]string, (i + 1))
		for k := 0; k < i+1; k++ {
			keys = append(keys, ids.GenerateTestID().String())
		}
		txID := ids.GenerateTestID()
		// Since these are all different keys, we will
		// fetch each key from disk
		require.NoError(f.Fetch(ctx, txID, keys))
		go func() {
			defer wg.Done()
			// Get keys from cache
			storage, err := f.Get(txID)
			require.NoError(err)

			// Add to cache
			l.Lock()
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
		}()
	}
	wg.Wait()
	require.NoError(f.Wait())
	require.Empty(cache)
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
		keys := make([]string, (i + 1))
		for k := 0; k < i+1; k++ {
			keys = append(keys, keyBase+strconv.Itoa(k))
		}
		txID := ids.GenerateTestID()
		// We are fetching the same keys, so we should
		// be getting subsequent requests from cache
		require.NoError(f.Fetch(ctx, txID, keys))
		go func() {
			defer wg.Done()
			storage, err := f.Get(txID)
			require.NoError(err)
			l.Lock()
			for k := range storage {
				cache.Add(k)
			}
			l.Unlock()
		}()
	}
	wg.Wait()
	require.NoError(f.Wait())
	require.Len(cache, 50)
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
		keys := make([]string, (i + 1))
		for k := 0; k < i+1; k++ {
			keys = append(keys, keyBase+strconv.Itoa(k))
		}
		txID := ids.GenerateTestID()

		// Empty chan to mimic timing out
		delay := make(chan struct{})

		// Fetch the key
		require.NoError(f.Fetch(ctx, txID, keys))
		go func() {
			defer wg.Done()
			// Get the keys from cache
			storage, err := f.Get(txID)
			require.NoError(err)

			// Notify we're done
			close(delay)
			l.Lock()
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
	require.Len(cache, 50)
}

func TestFetcherStop(t *testing.T) {
	var (
		require = require.New(t)
		numTxs  = 100
		f       = New(newTestDB(), numTxs, 10)
		ctx     = context.TODO()
		wg      sync.WaitGroup

		l     sync.Mutex
		cache = set.Set[string]{}
	)
	wg.Add(numTxs)
	for i := 0; i < numTxs; i++ {
		keys := make([]string, (i + 1))
		for k := 0; k < i+1; k++ {
			keys = append(keys, keyBase+strconv.Itoa(k))
		}
		txID := ids.GenerateTestID()
		err := f.Fetch(ctx, txID, keys)
		if err != nil {
			// Some [Fetch] may return an error.
			// This happens after we called [Stop]
			require.Equal(ErrStopped, err)
		}
		go func(i int) {
			defer wg.Done()
			storage, err := f.Get(txID)
			if err != nil {
				return
			}
			l.Lock()
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
