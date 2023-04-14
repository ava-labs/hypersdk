// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tstate

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"

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

func TestSetGetValue(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	tdb := NewTestDB()
	ts := New(tdb, 10)

	// Sets scope
	ts.SetScope(ctx, [][]byte{TestKey}, map[string][]byte{string(TestKey): TestVal})
	v, err := ts.GetValue(ctx, []byte("other"))
	require.ErrorIs(err, ErrKeyNotSpecified)
	require.Nil(v)

	// Adds key/value
	difVal := []byte("difVal")
	require.NoError(ts.Insert(ctx, TestKey, difVal), "error during insert")

	// Ensure value added to database
	require.Equal(1, ts.OpIndex(), "operation not added")
	v, err = tdb.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(difVal, v)
	require.Nil(ts.storage[string(TestKey)], "original value was not deleted from storage")
}

func TestGetValueNoStorage(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(nil, 10) // db should not be needed

	// SetScope but dont add to storage
	ts.SetScope(ctx, [][]byte{TestKey}, map[string][]byte{})
	_, err := ts.GetValue(ctx, TestKey)
	require.ErrorIs(database.ErrNotFound, err, "No error thrown.")
}

func TestInsertNew(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	tdb := NewTestDB()
	ts := New(tdb, 10)

	// Insert before SetScope
	err := ts.Insert(ctx, TestKey, TestVal)
	require.ErrorIs(ErrKeyNotSpecified, err, "No error thrown.")

	// SetScope
	ts.SetScope(ctx, [][]byte{TestKey}, map[string][]byte{})

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
	tdb := NewTestDB()
	ts := New(tdb, 10)

	// SetScope and add
	ts.SetScope(ctx, [][]byte{TestKey}, map[string][]byte{string(TestKey): TestVal})
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
}

func TestFetchAndSetScope(t *testing.T) {
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)

	// Add keys to parent
	pdb := NewTestDB()
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	for i, key := range keys {
		err := pdb.Insert(ctx, key, vals[i])
		require.NoError(err, "Error during insert.")
	}
	err := ts.FetchAndSetScope(ctx, keys, pdb)
	require.NoError(err, "Error thrown.")
	require.Equal(0, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(keys, ts.scope, "Scope not updated correctly.")

	// Check values
	for i, key := range keys {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
}

func TestFetchAndSetScopeMissingKey(t *testing.T) {
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)

	pdb := NewTestDB()
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	// Keys[3] not in db
	for i, key := range keys[:len(keys)-1] {
		err := pdb.Insert(ctx, key, vals[i])
		require.NoError(err, "Error during insert.")
	}
	err := ts.FetchAndSetScope(ctx, keys, pdb)
	require.NoError(err, "Error thrown.")
	require.Equal(0, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(keys, ts.scope, "Scope not updated correctly.")
	// Check values
	for i, key := range keys[:len(keys)-1] {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
	_, err = ts.GetValue(ctx, keys[2])
	require.ErrorIs(err, database.ErrNotFound, "Didn't throw correct erro.")
}

func TestRemove(t *testing.T) {
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)
	ctx := context.TODO()

	// Insert
	ts.SetScope(ctx, [][]byte{TestKey}, map[string][]byte{})
	err := ts.Insert(ctx, TestKey, TestVal)
	require.NoError(err, "Error from insert.")
	require.Equal(1, ts.OpIndex(), "Opertions not updated correctly.")

	// Remove
	err = ts.Remove(ctx, TestKey)
	require.NoError(err, "Error from remove.")
	_, ok := ts.storage[string(TestKey)]
	require.False(ok, "Key not deleted from storage.")
	require.Equal(2, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(TestVal, ts.ops[1].pastV, "Past value not set correctly.")
	require.Equal(remove, ts.ops[1].action, "Action not set correctly.")
	require.Equal(TestKey, ts.ops[1].k, "Action not set correctly.")
}

func TestRemoveNotInScope(t *testing.T) {
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)
	ctx := context.TODO()

	// Remove
	err := ts.Remove(ctx, TestKey)
	require.ErrorIs(err, ErrKeyNotSpecified, "ErrKeyNotSpecified should be thrown.")
}

func TestRemoveNotInStorage(t *testing.T) {
	ctx := context.TODO()
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)

	// Remove a key that is not in storage
	ts.SetScope(ctx, [][]byte{TestKey}, map[string][]byte{})
	err := ts.Remove(ctx, TestKey)
	require.NoError(err, "Error from remove.")
}

func TestRestoreInsert(t *testing.T) {
	ctx := context.TODO()
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)

	// Add values
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keys, map[string][]byte{})
	for i, key := range keys {
		err := ts.Insert(ctx, key, vals[i])
		require.NoError(err, "Error inserting.")
	}
	updatedVal := []byte("newVal")
	err := ts.Insert(ctx, keys[0], updatedVal)
	require.NoError(err, "Error inserting.")
	require.Equal(len(keys)+1, ts.OpIndex(), "Operations not added properly.")

	// Check value of first key
	val, err := ts.GetValue(ctx, keys[0])
	require.NoError(err, "Error getting value.")
	require.Equal(updatedVal, val, "Value not updated correctly.")

	// Rollback inserting updatedVal and key[2]
	ts.Rollback(ctx, 2)
	require.Equal(2, ts.OpIndex(), "Operations not rolled back properly.")
	// Keys[2] was removed
	_, ok := ts.storage[string(keys[2])]
	require.False(ok, "TState read op not rolled back properly.")
	// Keys[0] was set to past value
	val, err = ts.GetValue(ctx, keys[0])
	require.False(ok, "TState read op not rolled back properly.")
	require.NoError(err, "Error getting value.")
	require.Equal(vals[0], val, "Value not rolled back properly.")
}

func TestRestoreDelete(t *testing.T) {
	require := require.New(t)
	tdb := NewTestDB()
	ts := New(tdb, 10)
	ctx := context.TODO()

	// Add
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keys, map[string][]byte{
		string(keys[0]): vals[0],
		string(keys[1]): vals[1],
		string(keys[2]): vals[2],
	})
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
	require.Equal(3, ts.OpIndex(), "Operations not added properly.")

	// Roll back all removes
	ts.Rollback(ctx, 0)
	require.Equal(0, ts.OpIndex(), "Operations not rolled back properly.")
	for i, key := range keys {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not reset correctly.")
	}
}
