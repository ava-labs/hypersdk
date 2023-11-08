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

type opType uint8

const (
	createOp opType = 0
	insertOp opType = 1
	removeOp opType = 2
)

type op struct {
	t               opType
	k               string
	pastV           []byte
	pastAllocations *uint16
	pastWrites      *uint16
}

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

		switch op.t {
		case createOp:
			delete(ts.allocations, op.k)
			if op.pastWrites != nil {
				// If previously deleted value, we need to restore
				// that modification.
				ts.writes[op.k] = *op.pastWrites // must be 0 (delete)
				ts.pendingChangedKeys[op.k] = maybe.Nothing[[]byte]()
			} else {
				// If this was the first time we were writing to the key,
				// we can remove the record of that write.
				delete(ts.writes, op.k)
				delete(ts.pendingChangedKeys, op.k)
			}
		case insertOp:
			if op.pastWrites != nil {
				// If we previously wrote to the key in this view,
				// we should restore that.
				ts.writes[op.k] = *op.pastWrites
				ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
			} else {
				// If this was our first time writing to the key,
				// we should remove record of that write.
				delete(ts.writes, op.k)
				delete(ts.pendingChangedKeys, op.k)
			}
		case removeOp:
			if op.pastAllocations != nil {
				// If we removed a newly created key, we need to restore it
				// to [allocations].
				//
				// The key should have already been removed from allocations,
				// so we don't need to explicitly delete it if [op.pastAllocations]
				// is nil.
				ts.allocations[op.k] = *op.pastAllocations
			}
			if op.pastWrites != nil {
				// If we removed a newly written key, we should revert
				// it back to its previous value.
				ts.writes[op.k] = *op.pastWrites
				ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
			} else {
				// If we removed a key that already existed before the view,
				// we should remove it from tracking.
				delete(ts.writes, op.k)
				delete(ts.pendingChangedKeys, op.k)
			}
		}
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
	v, exists := ts.getValue(ctx, k)
	if !exists {
		return nil, database.ErrNotFound
	}
	return v, nil
}

func (ts *TStateView) getValue(ctx context.Context, key string) ([]byte, bool) {
	if v, ok := ts.pendingChangedKeys[key]; ok {
		if v.IsNothing() {
			return nil, false
		}
		return v.Value(), true
	}
	if v, changed, exists := ts.ts.getChangedValue(ctx, key); changed {
		return v, exists
	}
	if v, ok := ts.scopeStorage[key]; ok {
		return v, true
	}
	return nil, false
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
	valueChunks, _ := keys.NumChunks(value) // not possible to fail
	k := string(key)
	past, exists := ts.getValue(ctx, k)
	op := &op{
		k:               k,
		pastV:           past,
		pastAllocations: chunks(ts.allocations, k),
		pastWrites:      chunks(ts.writes, k),
	}
	if exists {
		op.t = insertOp
		ts.writes[k] = valueChunks // set to latest value
	} else {
		if !ts.canAllocate {
			return ErrAllocationDisabled
		}
		op.t = createOp
		keyChunks, _ := keys.MaxChunks(key) // not possible to fail
		ts.allocations[k] = keyChunks
		ts.writes[k] = valueChunks
	}
	ts.ops = append(ts.ops, op)
	ts.pendingChangedKeys[k] = maybe.Some(value)
	return nil
}

// Remove deletes a key-value pair from ts.storage.
func (ts *TStateView) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	k := string(key)
	past, exists := ts.getValue(ctx, k)
	if !exists {
		// We do not update writes if the key does not exist.
		return nil
	}
	ts.ops = append(ts.ops, &op{
		t:               removeOp,
		k:               k,
		pastV:           past,
		pastAllocations: chunks(ts.allocations, k),
		pastWrites:      chunks(ts.writes, k),
	})
	if _, ok := ts.allocations[k]; ok {
		// If delete after allocating in the same view, it is
		// as if nothing happened.
		delete(ts.allocations, k)
		delete(ts.writes, k)
	} else {
		// If this is not a new allocation, we mark as an
		// explicit delete.
		ts.writes[k] = 0
	}
	ts.pendingChangedKeys[k] = maybe.Nothing[[]byte]()
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
