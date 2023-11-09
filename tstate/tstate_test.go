// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/trace"

	"github.com/stretchr/testify/require"
)

var (
	testKey = []byte("key")
	testVal = []byte("value")

	key1    = keys.EncodeChunks([]byte("key1"), 1)
	key1str = string(key1)
	key2    = keys.EncodeChunks([]byte("key2"), 2)
	key2str = string(key2)
	key3    = keys.EncodeChunks([]byte("key3"), 3)
	key3str = string(key3)
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

func TestScope(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// No Scope
	tsv := ts.NewView(set.Set[string]{}, map[string][]byte{})
	val, err := tsv.GetValue(ctx, testKey)
	require.ErrorIs(ErrKeyNotSpecified, err)
	require.Nil(val)
	require.ErrorIs(ErrKeyNotSpecified, tsv.Insert(ctx, testKey, testVal))
	require.ErrorIs(ErrKeyNotSpecified, tsv.Remove(ctx, testKey))
}

func TestGetValue(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// Set Scope
	tsv := ts.NewView(set.Of(string(testKey)), map[string][]byte{string(testKey): testVal})
	val, err := tsv.GetValue(ctx, testKey)
	require.NoError(err, "unable to get value")
	require.Equal(testVal, val, "value was not saved correctly")
}

func TestDeleteCommitGet(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// Delete value
	tsv := ts.NewView(set.Of(string(testKey)), map[string][]byte{string(testKey): testVal})
	require.NoError(tsv.Remove(ctx, testKey))
	tsv.Commit()

	// Check deleted
	tsv = ts.NewView(set.Of(string(testKey)), map[string][]byte{string(testKey): testVal})
	val, err := tsv.GetValue(ctx, testKey)
	require.ErrorIs(err, database.ErrNotFound)
	require.Nil(val)
}

func TestGetValueNoStorage(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope but dont add to storage
	tsv := ts.NewView(set.Of(string(testKey)), map[string][]byte{})
	_, err := tsv.GetValue(ctx, testKey)
	require.ErrorIs(database.ErrNotFound, err, "data should not exist")
}

func TestInsertNew(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope
	tsv := ts.NewView(set.Of(string(testKey)), map[string][]byte{})

	// Test Disable Allocate
	tsv.DisableAllocation()
	require.ErrorIs(tsv.Insert(ctx, testKey, testVal), ErrAllocationDisabled)
	tsv.EnableAllocation()

	// Insert key
	require.NoError(tsv.Insert(ctx, testKey, testVal))
	val, err := tsv.GetValue(ctx, testKey)
	require.NoError(err)
	require.Equal(1, tsv.OpIndex(), "insert was not added as an operation")
	require.Equal(testVal, val, "value was not set correctly")

	// Check commit
	tsv.Commit()
	require.Equal(1, ts.OpIndex(), "insert was not added as an operation")
}

func TestInsertInvalid(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope
	key := binary.BigEndian.AppendUint16([]byte("hello"), 0)
	tsv := ts.NewView(set.Of(string(key)), map[string][]byte{})

	// Insert key
	require.ErrorIs(tsv.Insert(ctx, key, []byte("cool")), ErrInvalidKeyValue)

	// Get key value
	_, err := tsv.GetValue(ctx, key)
	require.ErrorIs(err, database.ErrNotFound)
}

func TestInsertUpdate(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope and add
	tsv := ts.NewView(set.Of(string(testKey)), map[string][]byte{string(testKey): testVal})
	require.Equal(0, ts.OpIndex())

	// Insert key
	newVal := []byte("newVal")
	require.NoError(tsv.Insert(ctx, testKey, newVal))
	val, err := tsv.GetValue(ctx, testKey)
	require.NoError(err)
	require.Equal(1, tsv.OpIndex(), "insert operation was not added")
	require.Equal(newVal, val, "value was not set correctly")
	require.Equal(testVal, tsv.ops[0].pastV)
	require.Nil(tsv.ops[0].pastAllocates)
	require.Nil(tsv.ops[0].pastWrites)

	// Check value after commit
	tsv.Commit()
	tsv = ts.NewView(set.Of(string(testKey)), map[string][]byte{string(testKey): testVal})
	val, err = tsv.GetValue(ctx, testKey)
	require.NoError(err)
	require.Equal(newVal, val, "value was not committed correctly")
}

func TestInsertRemoveInsert(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope and add
	tsv := ts.NewView(set.Of(key2str), map[string][]byte{})
	require.Equal(0, ts.OpIndex())

	// Insert key for first time
	require.NoError(tsv.Insert(ctx, key2, testVal))
	allocates, writes := tsv.KeyOperations()
	require.EqualValues(map[string]uint16{key2str: 2}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal))

	// Remove key
	require.NoError(tsv.Remove(ctx, key2))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)

	// Insert key again
	require.NoError(tsv.Insert(ctx, key2, testVal))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{key2str: 2}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal))

	// Modify key
	testVal2 := []byte("blah")
	require.NoError(tsv.Insert(ctx, key2, testVal2))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{key2str: 2}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Rollback modify
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{key2str: 2}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal))

	// Rollback second insert
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)

	// Rollback remove
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{key2str: 2}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal))

	// Rollback insert
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)
	require.Equal(0, tsv.OpIndex())

	// Remove empty should do nothing
	require.NoError(tsv.Remove(ctx, key2))
	require.Equal(0, tsv.OpIndex())

}

func TestModifyRemoveInsert(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope and add
	tsv := ts.NewView(set.Of(key2str), map[string][]byte{key2str: testVal})
	require.Equal(0, ts.OpIndex())

	// Modify existing key
	testVal2 := []byte("blah")
	require.NoError(tsv.Insert(ctx, key2, testVal2))
	allocates, writes := tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Remove modified key
	require.NoError(tsv.Remove(ctx, key2))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 0}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Nothing[[]byte]())

	// Insert key again (with original value)
	require.NoError(tsv.Insert(ctx, key2, testVal))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)

	// Rollback insert
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 0}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Nothing[[]byte]())

	// Rollback remove
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Rollback modify
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)
	require.Equal(0, tsv.OpIndex())
}

func TestModifyRevert(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope and add
	tsv := ts.NewView(set.Of(key2str), map[string][]byte{key2str: testVal})
	require.Equal(0, ts.OpIndex())

	// Modify existing key
	testVal2 := []byte("blah")
	require.NoError(tsv.Insert(ctx, key2, testVal2))
	allocates, writes := tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Revert modification
	require.NoError(tsv.Insert(ctx, key2, testVal))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)

	// Rollback revert modification
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Rollback modification
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)
	require.Equal(0, tsv.OpIndex())
}

func TestModifyModify(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	ts := New(10)

	// SetScope and add
	tsv := ts.NewView(set.Of(key2str), map[string][]byte{key2str: testVal})
	require.Equal(0, ts.OpIndex())

	// Modify existing key
	testVal2 := []byte("blah")
	require.NoError(tsv.Insert(ctx, key2, testVal2))
	allocates, writes := tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Perform same modification (no change)
	require.NoError(tsv.Insert(ctx, key2, testVal2))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Revert modification
	require.NoError(tsv.Insert(ctx, key2, testVal))
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)

	// Rollback revert modification
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{key2str: 1}, writes)
	require.Equal(tsv.pendingChangedKeys[key2str], maybe.Some(testVal2))

	// Rollback modification
	tsv.Rollback(ctx, tsv.OpIndex()-1)
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{}, allocates)
	require.EqualValues(map[string]uint16{}, writes)
	require.NotContains(tsv.pendingChangedKeys, key2str)
	require.Equal(0, tsv.OpIndex())
}

func TestRemoveInsertRollback(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()

	// Insert
	tsv := ts.NewView(set.Of(string(testKey)), map[string][]byte{})
	require.NoError(tsv.Insert(ctx, testKey, testVal))
	v, err := tsv.GetValue(ctx, testKey)
	require.NoError(err)
	require.Equal(testVal, v)
	require.Equal(1, tsv.OpIndex(), "opertions not updated correctly")

	// Remove
	require.NoError(tsv.Remove(ctx, testKey), "unable to remove testKey")
	_, err = tsv.GetValue(ctx, testKey)
	require.ErrorIs(err, database.ErrNotFound, "Key not deleted from storage")
	require.Equal(2, tsv.OpIndex(), "Opertions not updated correctly")

	// Insert
	require.NoError(tsv.Insert(ctx, testKey, testVal))
	v, err = tsv.GetValue(ctx, testKey)
	require.NoError(err)
	require.Equal(testVal, v)
	require.Equal(3, tsv.OpIndex(), "Opertions not updated correctly")
	require.Equal(1, tsv.PendingChanges())

	// Rollback
	tsv.Rollback(ctx, 2)
	_, err = tsv.GetValue(ctx, testKey)
	require.ErrorIs(err, database.ErrNotFound, "Key not deleted from storage")

	// Rollback
	tsv.Rollback(ctx, 1)
	v, err = tsv.GetValue(ctx, testKey)
	require.NoError(err)
	require.Equal(testVal, v)
}

func TestRestoreInsert(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	keys := [][]byte{key1, key2, key3}
	keySet := set.Of(key1str, key2str, key3str)
	vals := [][]byte{[]byte("val1"), []byte("val2"), []byte("val3")}

	// Store keys
	tsv := ts.NewView(keySet, map[string][]byte{})
	for i, key := range keys {
		require.NoError(tsv.Insert(ctx, key, vals[i]))
	}

	// Ensure KeyOperations reflect operations
	allocMap := map[string]uint16{key1str: 1, key2str: 2, key3str: 3}
	writeMap := map[string]uint16{key1str: 1, key2str: 1, key3str: 1}
	allocates, writes := tsv.KeyOperations()
	require.EqualValues(allocMap, allocates)
	require.EqualValues(writeMap, writes)

	// Update keys[0]
	updatedVal := []byte("newVal")
	require.NoError(tsv.Insert(ctx, keys[0], updatedVal))
	require.Equal(len(keys)+1, tsv.OpIndex(), "operations not added properly")
	val, err := tsv.GetValue(ctx, keys[0])
	require.NoError(err, "error getting value")
	require.Equal(updatedVal, val, "value not updated correctly")

	// No change to KeyOperations
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(allocMap, allocates)
	require.EqualValues(writeMap, writes)

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

	// Modifications rolled back
	allocates, writes = tsv.KeyOperations()
	require.EqualValues(map[string]uint16{key1str: 1, key2str: 2}, allocates)
	require.EqualValues(map[string]uint16{key1str: 1, key2str: 1}, writes)
}

func TestRestoreDelete(t *testing.T) {
	require := require.New(t)
	ts := New(10)
	ctx := context.TODO()
	keys := [][]byte{key1, key2, key3}
	keySet := set.Of(key1str, key2str, key3str)
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
	keys := [][]byte{key1, key2, key3}
	keySet := set.Of(key1str, key2str, key3str)
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

	// Check modifications
	allocMap := map[string]uint16{key1str: 1, key2str: 2, key3str: 3}
	writeMap := map[string]uint16{key1str: 1, key2str: 1, key3str: 1}
	allocates, writes := tsv.KeyOperations()
	require.EqualValues(allocMap, allocates)
	require.EqualValues(writeMap, writes)

	// Test warm modification
	tsvM := ts.NewView(keySet, map[string][]byte{})
	require.NoError(tsvM.Insert(ctx, keys[0], vals[2]))
	allocates, writes = tsvM.KeyOperations()
	require.Empty(allocates)
	require.EqualValues(map[string]uint16{key1str: 1}, writes)

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
