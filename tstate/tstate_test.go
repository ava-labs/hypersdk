// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
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

	// SetScope
	tsv := ts.NewView(set.Of(string(TestKey)), map[string][]byte{string(TestKey): TestVal})
	val, err := tsv.GetValue(ctx, TestKey)
	require.NoError(err, "unable to get value")
	require.Equal(TestVal, val, "value was not saved correctly")
}

func TestGetValueNoStorage(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope but dont add to storage
	tsv := ts.NewView(set.Of(string(TestKey)), map[string][]byte{})
	_, err := tsv.GetValue(ctx, TestKey)
	require.ErrorIs(database.ErrNotFound, err, "data should not exist")
}

func TestInsertNew(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope
	tsv := ts.NewView(set.Of(string(TestKey)), map[string][]byte{})

	// Insert key
	require.NoError(tsv.Insert(ctx, TestKey, TestVal))
	val, err := tsv.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(1, tsv.OpIndex(), "insert was not added as an operation")
	require.Equal(TestVal, val, "value was not set correctly")

	// Check commit
	tsv.Commit()
	require.Equal(1, ts.OpIndex(), "insert was not added as an operation")
}

func TestInsertUpdate(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope and add
	tsv := ts.NewView(set.Of(string(TestKey)), map[string][]byte{string(TestKey): TestVal})
	require.Equal(0, ts.OpIndex())

	// Insert key
	newVal := []byte("newVal")
	require.NoError(tsv.Insert(ctx, TestKey, newVal))
	val, err := tsv.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(1, tsv.OpIndex(), "insert operation was not added")
	require.Equal(newVal, val, "value was not set correctly")
	require.Equal(TestVal, tsv.ops[0].pastV, "PastVal was not set correctly")
	require.False(tsv.ops[0].pastChanged, "PastVal was not set correctly")
	require.True(tsv.ops[0].pastExists, "PastVal was not set correctly")

	// Check value after commit
	tsv.Commit()
	tsv = ts.NewView(set.Of(string(TestKey)), map[string][]byte{string(TestKey): TestVal})
	val, err = tsv.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(newVal, val, "value was not committed correctly")
}

func TestRemoveInsertRollback(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()

	// Insert
	tsv := ts.NewView(set.Of(string(TestKey)), map[string][]byte{})
	require.NoError(tsv.Insert(ctx, TestKey, TestVal))
	v, err := tsv.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(TestVal, v)
	require.Equal(1, tsv.OpIndex(), "opertions not updated correctly")

	// Remove
	require.NoError(tsv.Remove(ctx, TestKey), "unable to remove TestKey")
	_, err = tsv.GetValue(ctx, TestKey)
	require.ErrorIs(err, database.ErrNotFound, "Key not deleted from storage")
	require.Equal(2, tsv.OpIndex(), "Opertions not updated correctly")

	// Insert
	require.NoError(tsv.Insert(ctx, TestKey, TestVal))
	v, err = tsv.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(TestVal, v)
	require.Equal(3, tsv.OpIndex(), "Opertions not updated correctly")
	require.Equal(1, tsv.PendingChanges())

	// Rollback
	tsv.Rollback(ctx, 2)
	_, err = tsv.GetValue(ctx, TestKey)
	require.ErrorIs(err, database.ErrNotFound, "Key not deleted from storage")

	// Rollback
	tsv.Rollback(ctx, 1)
	v, err = tsv.GetValue(ctx, TestKey)
	require.NoError(err)
	require.Equal(TestVal, v)
}

func TestRestoreInsert(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	keySet := set.Of("key1", "key2", "key3")
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	tsv := ts.NewView(keySet, map[string][]byte{})
	for i, key := range keys {
		require.NoError(tsv.Insert(ctx, key, vals[i]))
	}
	updatedVal := []byte("newVal")
	require.NoError(tsv.Insert(ctx, keys[0], updatedVal))
	require.Equal(len(keys)+1, tsv.OpIndex(), "operations not added properly")
	val, err := tsv.GetValue(ctx, keys[0])
	require.NoError(err, "error getting value")
	require.Equal(updatedVal, val, "value not updated correctly")

	// Rollback inserting updatedVal and key[2]
	tsv.Rollback(ctx, 2)
	require.Equal(2, tsv.OpIndex(), "operations not rolled back properly")

	// Keys[2] was removed
	_, err = tsv.GetValue(ctx, keys[2])
	require.ErrorIs(err, database.ErrNotFound, "TState read op not rolled back properly")

	// Keys[0] was set to past value
	val, err = tsv.GetValue(ctx, keys[0])
	require.NoError(err, "error getting value")
	require.Equal(vals[0], val, "value not rolled back properly")
}

func TestRestoreDelete(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	keySet := set.Of("key1", "key2", "key3")
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}
	tsv := ts.NewView(keySet, map[string][]byte{
		string(keys[0]): vals[0],
		string(keys[1]): vals[1],
		string(keys[2]): vals[2],
	})

	// Check scope
	for i, key := range keys {
		val, err := tsv.GetValue(ctx, key)
		require.NoError(err, "error getting value")
		require.Equal(vals[i], val, "value not set correctly")
	}

	// Remove all
	for _, key := range keys {
		require.NoError(tsv.Remove(ctx, key), "error removing from ts")
		_, err := tsv.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "value not removed")
	}
	require.Equal(len(keys), tsv.OpIndex(), "operations not added properly")
	require.Equal(3, tsv.PendingChanges())

	// Roll back all removes
	tsv.Rollback(ctx, 0)
	require.Equal(0, ts.OpIndex(), "operations not rolled back properly")
	require.Equal(0, ts.PendingChanges())
	for i, key := range keys {
		val, err := tsv.GetValue(ctx, key)
		require.NoError(err, "error getting value")
		require.Equal(vals[i], val, "value not reset correctly")
	}
}

func TestCreateView(t *testing.T) {
	require := require.New(t)

	ctx := context.TODO()
	ts := New(10)
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	db, err := merkledb.New(ctx, memdb.New(), merkledb.Config{
		BranchFactor:              merkledb.BranchFactor16,
		HistoryLength:             100,
		EvictionBatchSize:         units.MiB,
		IntermediateNodeCacheSize: units.MiB,
		ValueNodeCacheSize:        units.MiB,
		Tracer:                    tracer,
	})
	if err != nil {
		t.Fatal(err)
	}
	keys := [][]byte{[]byte("key1"), []byte("key2"), []byte("key3")}
	keySet := set.Of("key1", "key2", "key3")
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}

	// Add
	tsv := ts.NewView(keySet, map[string][]byte{})
	for i, key := range keys {
		require.NoError(tsv.Insert(ctx, key, vals[i]), "error inserting value")
		val, err := tsv.GetValue(ctx, key)
		require.NoError(err, "error getting value")
		require.Equal(vals[i], val, "value not set correctly")
	}
	tsv.Commit()

	// Create merkle view
	view, err := ts.ExportMerkleDBView(ctx, tracer, db)
	require.NoError(err, "error writing changes")
	require.NoError(view.CommitToDB(ctx))

	// Check if db was updated correctly
	for i, key := range keys {
		val, _ := db.GetValue(ctx, key)
		require.Equal(vals[i], val, "value not updated in db")
	}

	// Remove
	ts = New(10)
	tsv = ts.NewView(keySet, map[string][]byte{
		string(keys[0]): vals[0],
		string(keys[1]): vals[1],
		string(keys[2]): vals[2],
	})
	for _, key := range keys {
		err := tsv.Remove(ctx, key)
		require.NoError(err, "error removing from ts")
		_, err = tsv.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "key not removed")
	}
	tsv.Commit()

	// Create merkle view
	view, err = tsv.ts.ExportMerkleDBView(ctx, tracer, db)
	require.NoError(err, "error writing changes")
	require.NoError(view.CommitToDB(ctx))

	// Check if db was updated correctly
	for _, key := range keys {
		_, err := db.GetValue(ctx, key)
		require.ErrorIs(err, database.ErrNotFound, "value not removed from db")
	}
}
