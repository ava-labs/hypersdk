// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"strconv"
	"testing"
	"time"
	//"fmt"

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
		f.Lookup(ctx, txID, stateKeys)
		f.TxnsToFetch[txID].Wait()
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
		f.Lookup(ctx, txID, stateKeys)
		f.TxnsToFetch[txID].Wait()
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
		f.Lookup(ctx, txID, stateKeys)
		if i%2 == 0 {
			time.Sleep(1 * time.Second)
		}
		f.TxnsToFetch[txID].Wait()
	}
	require.Equal(100, len(f.Cache))
}

func TestFetchKeysWithValues(t *testing.T) {
	var (
		require = require.New(t)
		f       = New(10, 4, newTestDBWithValue())
		ctx     = context.TODO()
	)
	for i := 0; i < 100; i++ {
		stateKeys := make(state.Keys, (i + 1))
		for k := 0; k < i+1; k++ {
			// Generate the same keys
			stateKeys.Add(strconv.Itoa(k), state.Read)
		}
		txID := ids.GenerateTestID()
		f.Lookup(ctx, txID, stateKeys)
		f.TxnsToFetch[txID].Wait()
	}
	require.Equal(100, len(f.Cache))
}
