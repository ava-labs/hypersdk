// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"bytes"
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type op struct {
	k string

	pastExists  bool
	pastV       []byte
	pastChanged bool
}

type tempStorage struct {
	v       []byte
	removed bool
}

type cacheItem struct {
	Value  []byte
	Exists bool
}

// TState defines a struct for storing temporary state.
type TState struct {
	changedKeys map[string]*tempStorage
	fetchCache  map[string]*cacheItem // in case we evict and want to re-fetch

	// We don't differentiate between read and write scope because it is very
	// uncommon for a user to write something without first reading what is
	// there.
	scope        [][]byte // stores a list of managed keys in the TState struct
	scopeStorage map[string][]byte

	// Ops is a record of all operations performed on [TState]. Tracking
	// operations allows for reverting state to a certain point-in-time.
	ops []*op
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(changedSize int) *TState {
	return &TState{
		changedKeys: make(map[string]*tempStorage, changedSize),

		fetchCache: map[string]*cacheItem{},

		ops: make([]*op, 0, changedSize),
	}
}

// GetValue returns the value associated from tempStorage with the
// associated [key]. If [key] does not exist in readScope or if it is not found
// in storage an error is returned.
func (ts *TState) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	if !ts.checkScope(ctx, key) {
		return nil, ErrKeyNotSpecified
	}
	k := string(key)
	v, _, exists := ts.getValue(ctx, k)
	if !exists {
		return nil, database.ErrNotFound
	}
	return v, nil
}

func (ts *TState) getValue(_ context.Context, key string) ([]byte, bool, bool) {
	if v, ok := ts.changedKeys[key]; ok {
		if v.removed {
			return nil, true, false
		}
		return v.v, true, true
	}
	v, ok := ts.scopeStorage[key]
	if !ok {
		return nil, false, false
	}
	return v, false, true
}

// FetchAndSetScope updates ts to include the [db] values associated with [keys].
// FetchAndSetScope then sets the scope of ts to [keys]. If a key exists in
// ts.fetchCache set the key's value to the value from cache.
func (ts *TState) FetchAndSetScope(ctx context.Context, keys [][]byte, db Database) error {
	ts.scopeStorage = map[string][]byte{}
	for _, key := range keys {
		k := string(key)
		if val, ok := ts.fetchCache[k]; ok {
			if val.Exists {
				ts.scopeStorage[k] = val.Value
			}
			continue
		}
		v, err := db.GetValue(ctx, key)
		if errors.Is(err, database.ErrNotFound) {
			ts.fetchCache[k] = &cacheItem{Exists: false}
			continue
		}
		if err != nil {
			return err
		}
		ts.fetchCache[k] = &cacheItem{Value: v, Exists: true}
		ts.scopeStorage[k] = v
	}
	ts.scope = keys
	return nil
}

// SetReadScope sets the readscope of ts to [keys].
func (ts *TState) SetScope(_ context.Context, keys [][]byte, storage map[string][]byte) {
	ts.scope = keys
	ts.scopeStorage = storage
}

// checkScope returns whether [k] is in ts.readScope.
func (ts *TState) checkScope(_ context.Context, k []byte) bool {
	for _, s := range ts.scope {
		// TODO: benchmark and see if creating map is worth overhead
		if bytes.Equal(k, s) {
			return true
		}
	}
	return false
}

// Insert sets or updates ts.storage[key] to equal {value, false}.
func (ts *TState) Insert(ctx context.Context, key []byte, value []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	ts.ops = append(ts.ops, &op{
		k:           k,
		pastExists:  exists,
		pastV:       past,
		pastChanged: changed,
	})
	ts.changedKeys[k] = &tempStorage{value, false}
	return nil
}

// Renove deletes a key-value pair from ts.storage.
func (ts *TState) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	if !exists {
		return nil
	}
	ts.ops = append(ts.ops, &op{
		k:           k,
		pastExists:  true,
		pastV:       past,
		pastChanged: changed,
	})
	ts.changedKeys[k] = &tempStorage{nil, true}
	return nil
}

// OpIndex returns the number of operations done on ts.
func (ts *TState) OpIndex() int {
	return len(ts.ops)
}

func (ts *TState) PendingChanges() int {
	return len(ts.changedKeys)
}

// Rollback restores the TState to before the ts.op[restorePoint] operation.
func (ts *TState) Rollback(_ context.Context, restorePoint int) {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]
		// insert: Modified key for the first time
		//
		// remove: Removed key that was modified for first time in run
		if !op.pastChanged {
			delete(ts.changedKeys, op.k)
			continue
		}
		// insert: Modified key for the nth time
		//
		// remove: Removed key that was previously modified in run
		ts.changedKeys[op.k] = &tempStorage{op.pastV, !op.pastExists}
	}
	ts.ops = ts.ops[:restorePoint]
}

// WriteChanges updates [db] to reflect changes in ts. Insert to [db] if
// key was added, or remove key if otherwise.
func (ts *TState) WriteChanges(
	ctx context.Context,
	db Database,
	t trace.Tracer, //nolint:interfacer
) error {
	ctx, span := t.Start(
		ctx, "TState.WriteChanges",
		oteltrace.WithAttributes(
			attribute.Int("items", len(ts.changedKeys)),
		),
	)
	defer span.End()

	for key, tstorage := range ts.changedKeys {
		if !tstorage.removed {
			if err := db.Insert(ctx, []byte(key), tstorage.v); err != nil {
				return err
			}
			continue
		}
		if err := db.Remove(ctx, []byte(key)); err != nil {
			return err
		}
	}
	return nil
}
