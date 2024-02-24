// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	_ "time"

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

/*func TestFetchDifferentKeys(t *testing.T) {
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
		wg.Wait()
	}
	// There should be 5050 different keys now in the cache
	require.Equal(5050, len(f.Cache))
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
		wg.Wait()
	}
	// There's only 100 keys
	require.Equal(100, len(f.Cache))
}

func TestFetchSameKeysSlow(t *testing.T) {
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
		wg := f.Lookup(ctx, txID, stateKeys)
		if i%2 == 0 {
			time.Sleep(1 * time.Second)
		}
		wg.Wait()
	}
	require.Equal(100, len(f.Cache))
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
		wg.Wait()
	}
	require.Equal(100, len(f.Cache))
}*/

func TestFetchSameKeyRepeatedly(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(10, 10, newTestDBWithValue())
		ctx     = context.TODO()
	)
	for i := 0; i < 10; i++ {
		stateKeys := make(state.Keys, 1)
		if i < 5 {
			stateKeys.Add(strconv.Itoa(1), state.Read)
		} else {
			stateKeys.Add(strconv.Itoa(2), state.Read)
		}
		txID := ids.GenerateTestID()
		wg := f.Lookup(ctx, txID, stateKeys)
		fmt.Printf("before %v\n", txID)
		wg.Wait()
		fmt.Printf("after %v\n\n", txID)
	}
	require.Equal(2, len(f.Cache))
}
