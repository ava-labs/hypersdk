// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/keys"
)

const defaultOps = 4

type TStateView struct {
	ts                 *TState
	pendingChangedKeys map[string]maybe.Maybe[[]byte]

	// Ops is a record of all operations performed on [TState]. Tracking
	// operations allows for reverting state to a certain point-in-time.
	ops []*op

	// We don't differentiate between read and write scope.
	scope        set.Set[string] // stores a list of managed keys in the TState struct
	scopeStorage map[string][]byte

	// Store which keys are modified and how large their values were.
	canAllocate bool
	allocations map[string]uint16
	writes      map[string]uint16
}

func (ts *TState) NewView(scope set.Set[string], storage map[string][]byte) *TStateView {
	return &TStateView{
		ts:                 ts,
		pendingChangedKeys: make(map[string]maybe.Maybe[[]byte], len(scope)),

		ops: make([]*op, 0, defaultOps),

		scope:        scope,
		scopeStorage: storage,

		canAllocate: true, // default to allowing allocation
		allocations: make(map[string]uint16, len(scope)),
		writes:      make(map[string]uint16, len(scope)),
	}
}

// Rollback restores the TState to the ts.op[restorePoint] operation.
func (ts *TStateView) Rollback(_ context.Context, restorePoint int) {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]

		// allocate+write/write -> base state
		//
		// Remove all key changes from the view if the key was not previously
		// modified.
		if !op.pastChanged {
			delete(ts.allocations, op.k)
			delete(ts.writes, op.k)
			delete(ts.pendingChangedKeys, op.k)
			continue
		}

		// allocate+write -> nothing (previously deleted -> during block?)
		//
		// If a key did not previously exist, we remove any allocations
		// and ensure [ts.writes] is set to 0.
		if !op.pastExists {
			delete(ts.allocations, op.k)
			ts.writes[op.k] = 0
			ts.pendingChangedKeys[op.k] = maybe.Nothing[[]byte]()
			continue
		}

		// write -> last value
		//
		// If a key did previously exist, we set [ts.writes] to the
		// num chunks of [op.pastV].
		//
		// MaxChunks/NumChunks should never error because we previously
		// parsed [op.k] and [op.pastV]
		keyChunks, _ := keys.MaxChunks([]byte(op.k))
		valueChunks, _ := keys.NumChunks(op.pastV)
		ts.allocations[op.k] = keyChunks
		ts.writes[op.k] = valueChunks
		ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
	}
	ts.ops = ts.ops[:restorePoint]
}

// OpIndex returns the number of operations done on ts.
func (ts *TStateView) OpIndex() int {
	return len(ts.ops)
}

// DisableAllocation causes [Insert] to return an error if
// it would create a new key. This can be useful for constraining
// what a transaction can do during block execution (to allow for
// cheaper fees).
//
// Note, creation defaults to true.
func (ts *TStateView) DisableAllocation() {
	ts.canAllocate = false
}

// EnableAllocation removes the forcer error case in [Insert]
// if a new key is created.
//
// Note, creation defaults to true.
func (ts *TStateView) EnableAllocation() {
	ts.canAllocate = true
}

// KeyOperations returns the number of operations performed since the scope
// was last set.
//
// If an operation is performed more than once during this time, the largest
// operation will be returned here (if 1 chunk then 2 chunks are written to a key,
// this function will return 2 chunks).
func (ts *TStateView) KeyOperations() (map[string]uint16, map[string]uint16) {
	return ts.allocations, ts.writes
}

// checkScope returns whether [k] is in ts.readScope.
func (ts *TStateView) checkScope(_ context.Context, k []byte) bool {
	return ts.scope.Contains(string(k))
}

// GetValue returns the value associated from tempStorage with the
// associated [key]. If [key] does not exist in readScope or if it is not found
// in storage an error is returned.
func (ts *TStateView) GetValue(ctx context.Context, key []byte) ([]byte, error) {
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

// Exists returns whether or not the associated [key] is present.
func (ts *TStateView) Exists(ctx context.Context, key []byte) (bool, bool, error) {
	if !ts.checkScope(ctx, key) {
		return false, false, ErrKeyNotSpecified
	}
	k := string(key)
	_, changed, exists := ts.getValue(ctx, k)
	return changed, exists, nil
}

func (ts *TStateView) getValue(ctx context.Context, key string) ([]byte, bool, bool) {
	if v, ok := ts.pendingChangedKeys[key]; ok {
		if v.IsNothing() {
			return nil, true, false
		}
		return v.Value(), true, true
	}
	if v, changed, exists := ts.ts.getChangedValue(ctx, key); changed {
		return v, true, exists
	}
	if v, ok := ts.scopeStorage[key]; ok {
		return v, false, true
	}
	return nil, false, false
}

// Insert sets or updates ts.storage[key] to equal {value, false}.
//
// Any bytes passed into [Insert] will be consumed by [TState] and should
// not be modified/referenced after this call.
func (ts *TStateView) Insert(ctx context.Context, key []byte, value []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	if !keys.VerifyValue(key, value) {
		return ErrInvalidKeyValue
	}
	valueChunks, ok := keys.NumChunks(value)
	if !ok {
		return ErrInvalidKeyValue
	}
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	if exists {
		ts.writes[k] = valueChunks
	} else {
		if !ts.canAllocate {
			return ErrAllocationDisabled
		} else {
			keyChunks, ok := keys.MaxChunks(key)
			if !ok {
				// Should never happen because we check [MaxChunks] in [VerifyValue]
				return ErrInvalidKeyValue
			}
			ts.allocations[k] = keyChunks
			ts.writes[k] = valueChunks
		}
	}
	ts.pendingChangedKeys[k] = maybe.Some(value)
	ts.ops = append(ts.ops, &op{
		k: k,

		pastExists:  exists,
		pastV:       past,
		pastChanged: changed,
	})
	return nil
}

// Remove deletes a key-value pair from ts.storage.
func (ts *TStateView) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	if !exists {
		// We do not update writes if the key does not exist.
		return nil
	}
	delete(ts.allocations, k)
	ts.writes[k] = 0
	ts.pendingChangedKeys[k] = maybe.Nothing[[]byte]()
	ts.ops = append(ts.ops, &op{
		k: k,

		pastExists:  true,
		pastV:       past,
		pastChanged: changed,
	})
	return nil
}

func (ts *TStateView) PendingChanges() int {
	return len(ts.pendingChangedKeys)
}

func (ts *TStateView) Commit() {
	ts.ts.l.Lock()
	defer ts.ts.l.Unlock()

	for k, v := range ts.pendingChangedKeys {
		ts.ts.changedKeys[k] = v
	}
	ts.ts.ops += len(ts.ops)
}

// updateChunks sets the number of chunks associated with a key that will
// be returned in [KeyOperations]. We store the largest chunks used (within
// the limit specified by the key).
func updateChunks(m map[string]uint16, key string, value []byte) (*uint16, error) {
	chunks, ok := keys.NumChunks(value)
	if !ok {
		return nil, ErrInvalidKeyValue
	}
	previousChunks, ok := m[key]
	if !ok {
		m[key] = chunks
		return nil, nil
	}
	if chunks > previousChunks {
		m[key] = chunks
	}
	return &previousChunks, nil
}

// chunks gets the number of chunks for a key in [m]
// or returns nil.
func chunks(m map[string]uint16, key string) *uint16 {
	chunks, ok := m[key]
	if !ok {
		return nil
	}
	return &chunks
}
