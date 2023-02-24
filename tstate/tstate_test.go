// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tstate

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
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
	if _, ok := db.storage[string(key)]; !ok {
		return database.ErrNotFound
	}
	delete(db.storage, string(key))
	return nil
}

func TestSetStorage(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()

	ts := New(10, 10)
	// Adds key/value
	ts.SetStorage(ctx, TestKey, TestVal)
	require.Equal(1, ts.OpIndex(), "Operation not added.")
	// require.False(ts.storage[string(TestKey)].fromDB, "Value not from a DB.")
	require.Equal(TestVal, ts.storage[string(TestKey)].v, "Value was not saved correctly.")
}

func TestSetStorageKeyExists(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10, 10)
	// Adds key/value
	ts.SetStorage(ctx, TestKey, TestVal)
	require.Equal(1, ts.OpIndex(), "Operation not added.")
	require.Equal(TestVal, ts.storage[string(TestKey)].v, "Value was not saved correctly.")
	// Set a value with a different key.
	ts.SetStorage(ctx, TestKey, []byte("DifferentValue"))
	require.Equal(1, ts.OpIndex(), "Read operation was not added.")
	require.Equal(TestVal, ts.storage[string(TestKey)].v, "Value was not saved correctly.")
}

func TestSetStorageKeyModified(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10, 10)
	// Adds key/value
	ts.SetScope(ctx, [][]byte{TestKey})
	ts.SetStorage(ctx, TestKey, TestVal)
	// ChangedKey = true
	difVal := []byte("difVal")
	err := ts.Insert(ctx, TestKey, difVal)
	require.NoError(err, "Error during insert.")

	require.Equal(2, ts.OpIndex(), "Operation not added.")
	// Otherval should not be saved
	ts.SetStorage(ctx, TestKey, []byte("OtherValue"))
	require.Equal(difVal, ts.storage[string(TestKey)].v, "Value was not saved correctly.")
}

func TestGetValue(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10, 10)
	// Adds key/value
	ts.SetStorage(ctx, TestKey, TestVal)
	require.Equal(1, ts.OpIndex(), "Operation not added.")
	require.Equal(TestVal, ts.storage[string(TestKey)].v, "Value was not saved correctly.")
	_, err := ts.GetValue(ctx, TestKey)
	// GetValue without Scope perm
	require.ErrorIs(err, ErrKeyNotSpecified, "No error thrown.")
	// SetScope
	ts.SetScope(ctx, [][]byte{TestKey})
	val, err := ts.GetValue(ctx, TestKey)
	require.NoError(err, "Error getting value.")
	require.Equal(TestVal, val, "Value was not saved correctly.")
}

func TestGetValueNoStorage(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10, 10)
	// SetScope but dont add to storage
	ts.SetScope(ctx, [][]byte{TestKey})
	_, err := ts.GetValue(ctx, TestKey)
	require.ErrorIs(database.ErrNotFound, err, "No error thrown.")
}

func TestInsertNew(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10, 10)
	// Insert before SetScope
	err := ts.Insert(ctx, TestKey, TestVal)
	require.ErrorIs(ErrKeyNotSpecified, err, "No error thrown.")
	// SetScope
	ts.SetScope(ctx, [][]byte{TestKey})
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
	ts := New(10, 10)
	// SetScope and add
	ts.SetScope(ctx, [][]byte{TestKey})
	ts.SetStorage(ctx, TestKey, TestVal)
	require.Equal(1, ts.OpIndex(), "SetStorage operation was not added.")
	// Insert key
	newVal := []byte("newVal")
	err := ts.Insert(ctx, TestKey, newVal)
	require.NoError(err, "Error thrown.")
	val, err := ts.GetValue(ctx, TestKey)
	require.NoError(err, "Error thrown.")
	require.Equal(2, ts.OpIndex(), "Insert operation was not added.")
	require.Equal(newVal, val, "Value was not set correctly.")
	require.Equal(TestVal, ts.ops[1].pastV.v, "PastVal was not set correctly.")
}

func TestFetchAndSetScope(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	db := NewTestDB()
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	for i, key := range keys {
		err := db.Insert(ctx, key, vals[i])
		require.NoError(err, "Error during insert.")
	}
	err := ts.FetchAndSetScope(ctx, db, keys)
	require.NoError(err, "Error thrown.")
	require.Equal(len(keys), ts.OpIndex(), "Opertions not updated correctly.")
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
	ts := New(10, 10)
	db := NewTestDB()
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	// Keys[3] not in db
	for i, key := range keys[:len(keys)-1] {
		err := db.Insert(ctx, key, vals[i])
		require.NoError(err, "Error during insert.")
	}
	err := ts.FetchAndSetScope(ctx, db, keys)
	require.NoError(err, "Error thrown.")
	require.Equal(len(keys)-1, ts.OpIndex(), "Opertions not updated correctly.")
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

func TestSetScope(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	ts.SetScope(ctx, keys)
	require.Equal(keys, ts.scope, "Scope not updated correctly.")
}

func TestRemove(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	ctx := context.TODO()
	ts.SetScope(ctx, [][]byte{TestKey})
	// Insert
	err := ts.Insert(ctx, TestKey, TestVal)
	require.NoError(err, "Error from insert.")
	require.Equal(1, ts.OpIndex(), "Opertions not updated correctly.")
	// Remove
	err = ts.Remove(ctx, TestKey)
	require.NoError(err, "Error from remove.")
	_, ok := ts.storage[string(TestKey)]
	require.False(ok, "Key not deleted from storage.")
	require.Equal(2, ts.OpIndex(), "Opertions not updated correctly.")
	require.Equal(TestVal, ts.ops[1].pastV.v, "Past value not set correctly.")
	require.Equal(remove, ts.ops[1].action, "Action not set correctly.")
	require.Equal(TestKey, ts.ops[1].k, "Action not set correctly.")
}

func TestRemoveNotInScope(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	ctx := context.TODO()
	// Remove
	err := ts.Remove(ctx, TestKey)
	require.ErrorIs(err, ErrKeyNotSpecified, "ErrKeyNotSpecified should be thrown.")
}

func TestRemoveNotInStorage(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	ctx := context.TODO()
	ts.SetScope(ctx, [][]byte{TestKey})
	// Remove a key that is not in storage
	err := ts.Remove(ctx, TestKey)
	require.NoError(err, "Error from remove.")
}

func TestRestoreReads(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keys)
	// Keys[3] not in db
	for i, key := range keys {
		ts.SetStorage(ctx, key, vals[i])
	}
	require.Equal(len(keys), ts.OpIndex(), "Operations not added properly.")
	// Roll back last two reads
	ts.Rollback(ctx, 1)
	require.Equal(1, ts.OpIndex(), "Operations not rolled back properly.")
	for _, key := range keys[1:] {
		_, ok := ts.storage[string(key)]
		require.False(ok, "TState read op not rolled back properly.")
	}
	_, ok := ts.storage[string(keys[0])]
	require.True(ok, "Rollbacked too far.")
}

func TestRestoreInsert(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keys)
	// Keys[3] not in db
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
	ts := New(10, 10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keys)
	// Add
	for i, key := range keys {
		ts.SetStorage(ctx, key, vals[i])
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
	require.Equal(len(keys)*2, ts.OpIndex(), "Operations not added properly.")
	// Roll back all removes
	ts.Rollback(ctx, 3)
	require.Equal(3, ts.OpIndex(), "Operations not rolled back properly.")
	for i, key := range keys {
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not reset correctly.")
	}
}

func TestWriteChanges(t *testing.T) {
	require := require.New(t)
	ts := New(10, 10)
	db := NewTestDB()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	ts.SetScope(ctx, keys)

	// Add
	for i, key := range keys {
		err := ts.Insert(ctx, key, vals[i])
		require.NoError(err, "Error inserting value.")
		val, err := ts.GetValue(ctx, key)
		require.NoError(err, "Error getting value.")
		require.Equal(vals[i], val, "Value not set correctly.")
	}
	err := ts.WriteChanges(ctx, db, tracer)
	require.NoError(err, "Error writing changes.")

	// Check if db was updated correctly
	for i, key := range keys {
		val, _ := db.GetValue(ctx, key)
		require.Equal(vals[i], val, "Value not updated in db.")
	}
	// Remove
	for _, key := range keys {
		err := ts.Remove(ctx, key)
		require.NoError(err, "Error removing from ts.")

		_, err = ts.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "Key not removed.")
	}

	err = ts.WriteChanges(ctx, db, tracer)
	require.NoError(err, "Error writing changes.")
	// Check if db was updated correctly
	for _, key := range keys {
		_, err := db.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "Value not removed from db.")
	}
}
