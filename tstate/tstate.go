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
	read opAction = iota
	insert
	remove
)

type op struct {
	action opAction
	k      []byte
	v      []byte
	pastV  *tempStorage
}

type tempStorage struct {
	v      []byte
	fromDB bool
}

type cacheItem struct {
	Value  []byte
	Exists bool
}

// TState defines a struct for storing temporary state.
type TState struct {
	// We use pointers here because tempStorage objects may be added/removed from
	// ops frequently. It is more efficient to avoid reallocating state each time
	// this happens.
	storage     map[string]*tempStorage
	changedKeys map[string]bool // Stores if key in [storage] was ever changed.

	fetchCache map[string]*cacheItem // in case we evict and want to re-fetch

	// We don't differentiate between read and write scope because it is very
	// uncommon for a user to write something without first reading what is
	// there.
	scope [][]byte // Stores a list of managed keys in the TState struct.

	// Ops is a record of all operations performed on [TState]. Tracking
	// operations allows for reverting state to a certain point-in-time.
	ops []*op
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(storageSize int, changedSize int) *TState {
	return &TState{
		storage:     make(map[string]*tempStorage, storageSize),
		changedKeys: make(map[string]bool, changedSize),

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
	v, ok := ts.storage[string(key)]
	if ok {
		return v.v, nil
	}
	return nil, database.ErrNotFound
}

// FetchAndSetScope updates ts to include the [db] values associated with [keys].
// FetchAndSetScope then sets the scope of ts to [keys]. If a key exists in
// ts.fetchCache set the key's value to the value from cache.
func (ts *TState) FetchAndSetScope(ctx context.Context, db Database, keys [][]byte) error {
	for _, key := range keys {
		k := string(key)
		if val, ok := ts.fetchCache[k]; ok {
			if val.Exists {
				ts.SetStorage(ctx, key, val.Value)
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
		ts.SetStorage(ctx, key, v)
	}
	ts.SetScope(ctx, keys)
	return nil
}

// SetStorage sets ts.storage[key] = {value, true}. Does not add to storage if
// ts already stores a mapping with key, or that key was previously modified.
func (ts *TState) SetStorage(_ context.Context, key []byte, value []byte) {
	k := string(key)
	if _, ok := ts.storage[k]; ok {
		// Don't double store info (2 txs could've reference)
		return
	}
	if _, ok := ts.changedKeys[k]; ok {
		// Don't overwrite if previously modified (tx could've deleted value
		// previously)
		return
	}

	// Populate rollback (note, we only care if an item was placed in storage
	// initially)
	ts.ops = append(ts.ops, &op{
		action: read,
		k:      key,
	})
	ts.storage[k] = &tempStorage{value, true}
}

// SetReadScope sets the readscope of ts to [keys].
func (ts *TState) SetScope(_ context.Context, keys [][]byte) {
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
	k := string(key)

	// Populate rollback
	ts.ops = append(ts.ops, &op{
		action: insert,
		k:      key,
		v:      value,
		pastV:  ts.storage[k],
	})

	ts.storage[k] = &tempStorage{value, false}
	ts.changedKeys[k] = true
	return nil
}

// Renove deletes a key-value pair from ts.storage.
func (ts *TState) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	k := string(key)

	// Populate rollback
	ts.ops = append(ts.ops, &op{
		action: remove,
		k:      key,
		pastV:  ts.storage[k],
	})

	delete(ts.storage, k)
	ts.changedKeys[k] = false
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
		case read:
			delete(ts.storage, k)
			delete(ts.changedKeys, k)
		case insert:
			if pv := op.pastV; pv != nil {
				// Key previously inserted
				if pv.fromDB {
					delete(ts.changedKeys, k)
				}
				ts.storage[k] = pv
			} else {
				// Key inserted for the first time
				delete(ts.storage, k)
				delete(ts.changedKeys, k)
			}
		case remove:
			if pv := op.pastV; pv != nil {
				// Key previously inserted
				if pv.fromDB {
					delete(ts.changedKeys, k)
				}
				ts.storage[k] = pv
			} else {
				// Key deleted for the first time
				delete(ts.changedKeys, k)
			}
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

	for key, added := range ts.changedKeys {
		if added {
			v := ts.storage[key]
			if err := db.Insert(ctx, []byte(key), v.v); err != nil {
				return err
			}
		} else {
			if err := db.Remove(ctx, []byte(key)); err != nil {
				return err
			}
		}
	}
	return nil
}
