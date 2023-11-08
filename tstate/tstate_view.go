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

	// Store which keys are modified and how large their values were. Reset
	// whenever setting scope.
	canAllocate bool
	allocations map[string]uint16
	writes      map[string]uint16
}

func (ts *TState) NewView(scope set.Set[string], storage map[string][]byte) *TStateView {
	return &TStateView{
		ts:                 ts,
		pendingChangedKeys: make(map[string]maybe.Maybe[[]byte], len(scope)),
		ops:                make([]*op, 0, defaultOps),
		scope:              scope,
		scopeStorage:       storage,
		canAllocate:        true, // default to allowing allocation
		allocations:        make(map[string]uint16, len(scope)),
		writes:             make(map[string]uint16, len(scope)),
	}
}

// Rollback restores the TState to the ts.op[restorePoint] operation.
func (ts *TStateView) Rollback(_ context.Context, restorePoint int) {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]

		// Handle rollbacks for allocations/writes
		if op.allocationsPastChunks != nil {
			ts.allocations[op.k] = *op.allocationsPastChunks
		} else {
			delete(ts.allocations, op.k)
		}
		if op.writePastChunks != nil {
			ts.writes[op.k] = *op.writePastChunks
		} else {
			delete(ts.writes, op.k)
		}

		// insert: Modified key for the first time
		//
		// remove: Removed key that was modified for first time in run
		if !op.pastChanged {
			delete(ts.pendingChangedKeys, op.k)
			continue
		}

		// insert: Modified key for the nth time
		//
		// remove: Removed key that was previously modified in run
		if !op.pastExists {
			ts.pendingChangedKeys[op.k] = maybe.Nothing[[]byte]()
		} else {
			ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
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
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	var (
		allocationsPastChunks, writePastChunks *uint16
		err                                    error
	)
	if exists {
		apc, ok := ts.allocations[k]
		if ok {
			allocationsPastChunks = &apc
		}
		writePastChunks, err = updateChunks(ts.writes, k, value)
	} else {
		if !ts.canAllocate {
			err = ErrAllocationDisabled
		} else {
			allocationsPastChunks, err = updateChunks(ts.allocations, k, value)
			if err == nil {
				writePastChunks, err = updateChunks(ts.writes, k, value)
			}
		}
	}
	if err != nil {
		return err
	}
	ts.ops = append(ts.ops, &op{
		k: k,

		pastExists:  exists,
		pastV:       past,
		pastChanged: changed,

		allocationsPastChunks: allocationsPastChunks,
		writePastChunks:       writePastChunks,
	})
	ts.pendingChangedKeys[k] = maybe.Some(value)
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
	var allocationsPastChunks *uint16
	apc, ok := ts.allocations[k]
	if ok {
		allocationsPastChunks = &apc
	}
	writePastChunks, err := updateChunks(ts.writes, k, nil)
	if err != nil {
		return err
	}
	ts.ops = append(ts.ops, &op{
		k: k,

		pastExists:  true,
		pastV:       past,
		pastChanged: changed,

		// TODO: handle this in rollback instead of fetching here
		allocationsPastChunks: allocationsPastChunks,
		writePastChunks:       writePastChunks,
	})
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
// be returned in [KeyOperations].
func updateChunks(m map[string]uint16, key string, value []byte) (*uint16, error) {
	chunks, ok := keys.NumChunks(value)
	if !ok {
		return nil, ErrInvalidKeyValue
	}
	previousChunks, ok := m[key]
	if ok {
		if chunks > previousChunks {
			m[key] = chunks
		}
		return &previousChunks, nil
	}
	m[key] = chunks
	return nil, nil
}
