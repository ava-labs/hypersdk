// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/trace"

	"github.com/stretchr/testify/require"
)

var (
	TestKey = []byte("key")
	TestVal = []byte("value")
)

type TestDB struct {
	storage map[string][]byte
}

func NewTestDB() *TestDB {
	return &TestDB{
		storage: make(map[string][]byte),
	}
}

func (db *TestDB) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	val, ok := db.storage[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return val, nil
}

func (db *TestDB) Insert(_ context.Context, key []byte, value []byte) error {
	db.storage[string(key)] = value
	return nil
}

func (db *TestDB) Remove(_ context.Context, key []byte) error {
	delete(db.storage, string(key))
	return nil
}

func TestGetValue(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)
	// GetValue without Scope perm
	_, err := ts.GetValue(ctx, TestKey)
	require.ErrorIs(err, ErrKeyNotSpecified, "No error thrown.")
	// SetScope
	ts.SetScope(ctx, set.Of(string(TestKey)), map[string][]byte{string(TestKey): TestVal})
	val, err := ts.GetValue(ctx, TestKey)
	require.NoError(err, "Error getting value.")
	require.Equal(TestVal, val, "Value was not saved correctly.")
}

func TestGetValueNoStorage(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)
	// SetScope but dont add to storage
	ts.SetScope(ctx, set.Of(string(TestKey)), map[string][]byte{})
	_, err := ts.GetValue(ctx, TestKey)
	require.ErrorIs(database.ErrNotFound, err, "No error thrown.")
}

func TestInsertNew(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)
	// Insert before SetScope
	err := ts.Insert(ctx, TestKey, TestVal)
	require.ErrorIs(ErrKeyNotSpecified, err, "No error thrown.")
	// SetScope
	ts.SetScope(ctx, set.Of(string(TestKey)), map[string][]byte{})
	// Insert key
	err = ts.Insert(ctx, TestKey, TestVal)
	require.NoError(err, "Error thrown.")
	val, err := ts.GetValue(ctx, TestKey)
	require.NoError(err, "Error thrown.")
	require.Equal(1, ts.OpIndex(), "Insert operation was not added.")
	require.Equal(TestVal, val, "Value was not set correctly.")
}

func TestInsertUpdate(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)
	// SetScope and add
	ts.SetScope(ctx, set.Of(string(TestKey)), map[string][]byte{string(TestKey): TestVal})
	require.Equal(0, ts.OpIndex(), "SetStorage operation was not added.")
	// Insert key
	newVal := []byte("newVal")
	err := ts.Insert(ctx, TestKey, newVal)
	require.NoError(err, "Error thrown.")
	val, err := ts.GetValue(ctx, TestKey)
	require.NoError(err, "Error thrown.")
	require.Equal(1, ts.OpIndex(), "Insert operation was not added.")
	require.Equal(newVal, val, "Value was not set correctly.")
	require.Equal(TestVal, ts.ops[0].pastV, "PastVal was not set correctly.")
	require.False(ts.ops[0].pastChanged, "PastVal was not set correctly.")
	require.True(ts.ops[0].pastExists, "PastVal was not set correctly.")
}

func TestFetchAndSetScope(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	db := NewTestDB()
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	keySet := set.NewSet[string](3)
	for i, key := range keys {
		err := db.Insert(ctx, key, vals[i])
		require.NoError(err, "Error during insert.")
		keySet.Add(string(key))
	}
	err := ts.FetchAndSetScope(ctx, keySet, db)
	require.NoError(err, "Error thrown.")
	require.Equal(0, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(keySet, ts.scope, "Scope not updated correctly.")
	// Check values
	for i, key := range keys {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
}

func TestFetchAndSetScopeMissingKey(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	db := NewTestDB()
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	keySet := set.NewSet[string](3)
	// Keys[3] not in db
	for i, key := range keys[:len(keys)-1] {
		keySet.Add(string(key))
		err := db.Insert(ctx, key, vals[i])
		require.NoError(err, "Error during insert.")
	}
	keySet.Add("key3")
	err := ts.FetchAndSetScope(ctx, keySet, db)
	require.NoError(err, "Error thrown.")
	require.Equal(0, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(keySet, ts.scope, "Scope not updated correctly.")
	// Check values
	for i, key := range keys[:len(keys)-1] {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
	_, err = ts.GetValue(ctx, keys[2])
	require.ErrorIs(err, database.ErrNotFound, "Didn't throw correct erro.")
}

func TestRemoveInsertRollback(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	ts.SetScope(ctx, set.Of(string(TestKey)), map[string][]byte{})
	// Insert
	err := ts.Insert(ctx, TestKey, TestVal)
	require.NoError(err, "Error from insert.")
	v, err := ts.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(TestVal, v)
	require.Equal(1, ts.OpIndex(), "Opertions not updated correctly.")
	// Remove
	err = ts.Remove(ctx, TestKey)
	require.NoError(err, "Error from remove.")
	_, err = ts.GetValue(ctx, TestKey)
	require.ErrorIs(err, database.ErrNotFound, "Key not deleted from storage.")
	require.Equal(2, ts.OpIndex(), "Opertions not updated correctly.")
	// Insert
	err = ts.Insert(ctx, TestKey, TestVal)
	require.NoError(err, "Error from insert.")
	v, err = ts.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(TestVal, v)
	require.Equal(3, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(1, ts.PendingChanges())
	// Rollback
	ts.Rollback(ctx, 2)
	_, err = ts.GetValue(ctx, TestKey)
	require.ErrorIs(err, database.ErrNotFound, "Key not deleted from storage.")
	// Rollback
	ts.Rollback(ctx, 1)
	v, err = ts.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(TestVal, v)
}

func TestRemoveNotInScope(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	// Remove
	err := ts.Remove(ctx, TestKey)
	require.ErrorIs(err, ErrKeyNotSpecified, "ErrKeyNotSpecified should be thrown.")
}

func TestRestoreInsert(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	keySet := set.Of("key1", "key2", "key3")
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keySet, map[string][]byte{})
	for i, key := range keys {
		err := ts.Insert(ctx, key, vals[i])
		require.NoError(err, "Error inserting.")
	}
	updatedVal := []byte("newVal")
	err := ts.Insert(ctx, keys[0], updatedVal)
	require.NoError(err, "Error inserting.")
	require.Equal(len(keys)+1, ts.OpIndex(), "Operations not added properly.")
	val, err := ts.GetValue(ctx, keys[0])
	require.NoError(err, "Error getting value.")
	require.Equal(updatedVal, val, "Value not updated correctly.")
	// Rollback inserting updatedVal and key[2]
	ts.Rollback(ctx, 2)
	require.Equal(2, ts.OpIndex(), "Operations not rolled back properly.")
	// Keys[2] was removed
	_, err = ts.GetValue(ctx, keys[2])
	require.ErrorIs(err, database.ErrNotFound, "TState read op not rolled back properly.")
	// Keys[0] was set to past value
	val, err = ts.GetValue(ctx, keys[0])
	require.NoError(err, "Error getting value.")
	require.Equal(vals[0], val, "Value not rolled back properly.")
}

func TestRestoreDelete(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	keySet := set.Of("key1", "key2", "key3")
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keySet, map[string][]byte{
		string(keys[0]): vals[0],
		string(keys[1]): vals[1],
		string(keys[2]): vals[2],
	})
	// Check scope
	for i, key := range keys {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
	// Remove all
	for _, key := range keys {
		err := ts.Remove(ctx, key)
		require.NoError(err, "Error removing from ts.")
		_, err = ts.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "Value not removed.")
	}
	require.Equal(len(keys), ts.OpIndex(), "Operations not added properly.")
	require.Equal(3, ts.PendingChanges())
	// Roll back all removes
	ts.Rollback(ctx, 0)
	require.Equal(0, ts.OpIndex(), "Operations not rolled back properly.")
	require.Equal(0, ts.PendingChanges())
	for i, key := range keys {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not reset correctly.")
	}
}

func TestCreateView(t *testing.T) {
	require := require.New(t)

	ctx := context.TODO()
	ts := New(10)
	m := manager.NewMemDB(version.Semantic1_0_0)
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	db, err := merkledb.New(ctx, m.Current().Database, merkledb.Config{
		HistoryLength: 100,
		NodeCacheSize: 1_000,
		Tracer:        tracer,
	})
	if err != nil {
		t.Fatal(err)
	}
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	keySet := set.Of("key1", "key2", "key3")
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keySet, map[string][]byte{})
	// Add
	for i, key := range keys {
		err := ts.Insert(ctx, key, vals[i])
		require.NoError(err, "Error inserting value.")
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
	view, err := ts.CreateView(ctx, db, tracer)
	require.NoError(err, "Error writing changes.")
	require.NoError(view.CommitToDB(ctx))
	// Check if db was updated correctly
	for i, key := range keys {
		val, _ := db.GetValue(ctx, key)
		require.Equal(vals[i], val, "Value not updated in db.")
	}
	// Remove
	ts = New(10)
	ts.SetScope(ctx, keySet, map[string][]byte{
		string(keys[0]): vals[0],
		string(keys[1]): vals[1],
		string(keys[2]): vals[2],
	})
	for _, key := range keys {
		err := ts.Remove(ctx, key)
		require.NoError(err, "Error removing from ts.")
		_, err = ts.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "Key not removed.")
	}
	view, err = ts.CreateView(ctx, db, tracer)
	require.NoError(err, "Error writing changes.")
	require.NoError(view.CommitToDB(ctx))
	// Check if db was updated correctly
	for _, key := range keys {
		_, err := db.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "Value not removed from db.")
	}
}
