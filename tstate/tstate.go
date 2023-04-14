// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"bytes"
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/set"
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

type cacheItem struct {
	Value  []byte
	Exists bool
}

// TState defines a struct for storing temporary state.
type TState struct {
	state       Database
	changedKeys set.Set[string] // Stores if key in [storage] was ever changed.

	fetchCache map[string]*cacheItem // in case we evict and want to re-fetch

	// We don't differentiate between read and write scope because it is very
	// uncommon for a user to write something without first reading what is
	// there.
	scope   [][]byte // Stores a list of managed keys in the TState struct.
	storage map[string][]byte

	// Ops is a record of all operations performed on [TState]. Tracking
	// operations allows for reverting state to a certain point-in-time.
	ops []*op
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(state Database, changedSize int) *TState {
	return &TState{
		state:       state,
		changedKeys: set.NewSet[string](changedSize),

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
	if !ts.changedKeys.Contains(k) {
		v, ok := ts.storage[k]
		if ok {
			return v, nil
		}
		return nil, database.ErrNotFound
	}
	return ts.state.GetValue(ctx, key)
}

// FetchAndSetScope updates ts to include the [db] values associated with [keys].
// FetchAndSetScope then sets the scope of ts to [keys]. If a key exists in
// ts.fetchCache set the key's value to the value from cache.
func (ts *TState) FetchAndSetScope(ctx context.Context, keys [][]byte, parentState Database) error {
	ts.storage = map[string][]byte{}
	for _, key := range keys {
		k := string(key)
		if val, ok := ts.fetchCache[k]; ok {
			if val.Exists {
				ts.storage[k] = val.Value
			}
			continue
		}
		v, err := parentState.GetValue(ctx, key)
		if errors.Is(err, database.ErrNotFound) {
			ts.fetchCache[k] = &cacheItem{Exists: false}
			continue
		}
		if err != nil {
			return err
		}
		ts.fetchCache[k] = &cacheItem{Value: v, Exists: true}
		ts.storage[k] = v
	}
	ts.scope = keys
	return nil
}

// SetReadScope sets the readscope of ts to [keys].
func (ts *TState) SetScope(_ context.Context, keys [][]byte, storage map[string][]byte) {
	ts.storage = storage
	ts.scope = keys
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
		action: insert,

		k: key,
		v: value,

		pastExists:  err == nil,
		pastV:       past,
		pastChanged: ts.changedKeys.Contains(k),
	})
	delete(ts.storage, k)
	ts.changedKeys.Add(k)
	return ts.state.Insert(ctx, key, value)
}

func (ts *TState) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	past, err := ts.getValue(ctx, key)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return err
	}
	k := string(key)
	ts.ops = append(ts.ops, &op{
		action: remove,

		k: key,

		pastExists:  err == nil,
		pastV:       past,
		pastChanged: ts.changedKeys.Contains(k),
	})
	delete(ts.storage, k)
	ts.changedKeys.Add(k)
	return ts.state.Remove(ctx, key)
}

// OpIndex returns the number of operations done on ts.
func (ts *TState) OpIndex() int {
	return len(ts.ops)
}

// Rollback restores the TState to before the ts.op[restorePoint] operation.
func (ts *TState) Rollback(ctx context.Context, restorePoint int) error {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]
		k := string(op.k)
		switch op.action {
		case insert:
			// Created key during insert
			if !op.pastExists {
				ts.changedKeys.Remove(k)
				if err := ts.state.Remove(ctx, op.k); err != nil {
					return err
				}
				break
			}
			// Modified previous key
			pv := op.pastV
			if !op.pastChanged {
				ts.changedKeys.Remove(k)
				ts.storage[k] = pv
			}
			if err := ts.state.Insert(ctx, op.k, pv); err != nil {
				return err
			}
		case remove:
			// Deleted non-existent key
			if !op.pastExists {
				ts.changedKeys.Remove(k)
				break
			}
			// Deleted key that existed
			pv := op.pastV
			if !op.pastChanged {
				ts.changedKeys.Remove(k)
				ts.storage[k] = pv
			}
			if err := ts.state.Insert(ctx, op.k, pv); err != nil {
				return err
			}
		default:
			panic("invalid op")
		}
	}
	ts.ops = ts.ops[:restorePoint]
	return nil
}
