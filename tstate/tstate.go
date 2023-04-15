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

type opAction int

const (
	insert opAction = iota
	remove
)

type op struct {
	action opAction

	k []byte
	v []byte

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
	return ts.getValue(ctx, key)
}

func (ts *TState) getValue(ctx context.Context, key []byte) ([]byte, error) {
	k := string(key)
	if v, ok := ts.changedKeys[k]; ok {
		if v.removed {
			return nil, database.ErrNotFound
		}
		return v.v, nil
	}
	v, ok := ts.scopeStorage[k]
	if ok {
		return v, nil
	}
	return nil, database.ErrNotFound
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
	past, err := ts.getValue(ctx, key)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return err
	}
	k := string(key)
	ts.ops = append(ts.ops, &op{
		action:      insert,
		k:           key,
		v:           value,
		pastExists:  err == nil,
		pastV:       past,
		pastChanged: ts.changedKeys[k] != nil,
	})
	ts.changedKeys[k] = &tempStorage{value, false}
	return nil
}

// Renove deletes a key-value pair from ts.storage.
func (ts *TState) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	past, err := ts.getValue(ctx, key)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return err
	}
	if err != nil {
		// This value does not exist, there is nothing to remove
		return nil
	}
	k := string(key)
	ts.ops = append(ts.ops, &op{
		action:      remove,
		k:           key,
		pastExists:  true,
		pastV:       past,
		pastChanged: ts.changedKeys[k] != nil,
	})
	ts.changedKeys[k] = &tempStorage{nil, true}
	return nil
}

// OpIndex returns the number of operations done on ts.
func (ts *TState) OpIndex() int {
	return len(ts.ops)
}

// Rollback restores the TState to before the ts.op[restorePoint] operation.
func (ts *TState) Rollback(_ context.Context, restorePoint int) {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]
		k := string(op.k)
		switch op.action {
		case insert:
			// Created key during insert
			if !op.pastExists {
				delete(ts.changedKeys, k)
				break
			}
			// Modified previous key
			if !op.pastChanged {
				delete(ts.changedKeys, k)
				break
			}
			ts.changedKeys[k] = &tempStorage{op.pastV, false}
		case remove:
			// We always assume any ops refer to keys that existed at one point
			// (meaning that the removal of an empty object would be skipped, even if
			// that object was a changed key).
			if !op.pastChanged {
				delete(ts.changedKeys, k)
			}
			ts.changedKeys[k] = &tempStorage{op.pastV, false}
		default:
			panic("invalid op")
		}
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
