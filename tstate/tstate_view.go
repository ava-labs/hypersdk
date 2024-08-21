// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"bytes"
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/maybe"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

const defaultOps = 4

type opType uint8

const (
	createOp opType = 0
	insertOp opType = 1
	removeOp opType = 2
)

type op struct {
	t             opType
	k             string
	pastV         []byte
	pastAllocates *uint16
	pastWrites    *uint16
}

type TStateView struct {
	ts                 *TState
	pendingChangedKeys map[string]maybe.Maybe[[]byte]

	// Ops is a record of all operations performed on [TState]. Tracking
	// operations allows for reverting state to a certain point-in-time.
	ops []*op

	// stores a map of managed keys and its permissions in the TState struct
	// TODO: Need to handle read-only/write-only keys differently (won't prefetch a write
	// key, see issue below)
	// https://github.com/ava-labs/hypersdk/issues/709
	scope        state.Keys
	scopeStorage map[string][]byte

	metrics *utils.Metrics

	// Store which keys are modified and how large their values were.
	allocates map[string]uint16
	writes    map[string]uint16
}

func (ts *TState) NewView(scope state.Keys, storage map[string][]byte, metrics *utils.Metrics) *TStateView {
	return &TStateView{
		ts:                 ts,
		pendingChangedKeys: make(map[string]maybe.Maybe[[]byte], len(scope)),

		ops: make([]*op, 0, defaultOps),

		scope:        scope,
		scopeStorage: storage,

		metrics: metrics,

		allocates: make(map[string]uint16, len(scope)),
		writes:    make(map[string]uint16, len(scope)),
	}
}

// Rollback restores the TState to the ts.op[restorePoint] operation.
func (ts *TStateView) Rollback(_ context.Context, restorePoint int) {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]

		switch op.t {
		case createOp:
			ts.metrics.UpdateAllocOperations(ts.allocates[op.k], 0)
			delete(ts.allocates, op.k)
			if op.pastWrites != nil {
				// If previously deleted value, we need to restore
				// that modification.
				ts.metrics.UpdateWriteOperations(ts.writes[op.k], *op.pastWrites)
				ts.writes[op.k] = *op.pastWrites // must be 0 (delete)
				ts.pendingChangedKeys[op.k] = maybe.Nothing[[]byte]()
			} else {
				// If this was the first time we were writing to the key,
				// we can remove the record of that write.
				ts.metrics.UpdateWriteOperations(ts.writes[op.k], 0)
				delete(ts.writes, op.k)
				delete(ts.pendingChangedKeys, op.k)
			}
		case insertOp:
			if op.pastWrites != nil {
				// If we previously wrote to the key in this view,
				// we should restore that.
				ts.metrics.UpdateWriteOperations(ts.writes[op.k], *op.pastWrites)
				ts.writes[op.k] = *op.pastWrites
				ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
			} else {
				// If this was our first time writing to the key,
				// we should remove record of that write.
				ts.metrics.UpdateWriteOperations(ts.writes[op.k], 0)
				delete(ts.writes, op.k)
				delete(ts.pendingChangedKeys, op.k)
			}
		case removeOp:
			if op.pastAllocates != nil {
				// If we removed a newly created key, we need to restore it
				// to [allocates].
				//
				// The key should have already been removed from allocates,
				// so we don't need to explicitly delete it if [op.pastAllocates]
				// is nil.
				ts.metrics.UpdateAllocOperations(ts.allocates[op.k], *op.pastAllocates)
				ts.allocates[op.k] = *op.pastAllocates
			}
			if op.pastWrites != nil {
				// If we removed a newly written key, we should revert
				// it back to its previous value.
				ts.metrics.UpdateWriteOperations(ts.writes[op.k], *op.pastWrites)
				ts.writes[op.k] = *op.pastWrites
				ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
			} else {
				// If we removed a key that already existed before the view,
				// we should remove it from tracking.
				//
				// This should never happen if [op.pastAllocates] != nil.
				ts.metrics.UpdateWriteOperations(ts.writes[op.k], 0)
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

// KeyOperations returns the number of operations performed since the scope
// was last set.
//
// If an operation is performed more than once during this time, the largest
// operation will be returned here (if 1 chunk then 2 chunks are written to a key,
// this function will return 2 chunks).
//
// TODO: this function is no longer used but could be a useful metric
func (ts *TStateView) KeyOperations() (map[string]uint16, map[string]uint16) {
	return ts.allocates, ts.writes
}

// checkScope returns whether [k] is in scope and has appropriate permissions.
func (ts *TStateView) checkScope(_ context.Context, k []byte, perm state.Permissions) bool {
	return ts.scope[string(k)].Has(perm)
}

// GetValue returns the value associated from tempStorage with the
// associated [key]. If [key] does not exist in scope, or is not read/rw, or if it is not found
// in storage an error is returned.
func (ts *TStateView) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	// Getting a value requires a Read permission, so we pass state.Read
	if !ts.checkScope(ctx, key, state.Read) {
		return nil, ErrInvalidKeyOrPermission
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

// isUnchanged determines if a [key] is unchanged from the parent view (or
// scope if the parent is unchanged).
func (ts *TStateView) isUnchanged(ctx context.Context, key string, nval []byte, nexists bool) bool {
	if v, changed, exists := ts.ts.getChangedValue(ctx, key); changed {
		return !exists && !nexists || exists && nexists && bytes.Equal(v, nval)
	}
	if v, ok := ts.scopeStorage[key]; ok {
		return nexists && bytes.Equal(v, nval)
	}
	return !nexists
}

// Insert allocates and writes (or just writes) a new key to [tstate]. If this
// action returns the value of [key] to the parent view, it reverts any pending changes.
func (ts *TStateView) Insert(ctx context.Context, key []byte, value []byte) error {
	// Inserting requires a Write Permissions, so we pass state.Write
	if !ts.checkScope(ctx, key, state.Write) {
		return ErrInvalidKeyOrPermission
	}
	if !keys.VerifyValue(key, value) {
		return ErrInvalidKeyValue
	}
	valueChunks, _ := keys.NumChunks(value) // not possible to fail
	k := string(key)
	// Invariant: [getValue] is safe to call here because with [state.Write], it
	// will provide Read and Write access to the state
	past, exists := ts.getValue(ctx, k)
	op := &op{
		k:             k,
		pastV:         past,
		pastAllocates: chunks(ts.allocates, k),
		pastWrites:    chunks(ts.writes, k),
	}
	if exists {
		if bytes.Equal(past, value) {
			// No change, so this isn't an op.
			return nil
		}
		op.t = insertOp
		ts.metrics.UpdateWriteOperations(ts.writes[op.k], valueChunks)
		ts.writes[k] = valueChunks // set to latest value
	} else {
		// New entry requires Allocate
		// TODO: we assume any allocate is a write too, but we should
		// make this invariant more clear. Do we require Write,
		// Allocate|Write, and never Allocate alone?
		if !ts.checkScope(ctx, key, state.Allocate) {
			return ErrInvalidKeyOrPermission
		}
		op.t = createOp
		keyChunks, _ := keys.MaxChunks(key) // not possible to fail
		ts.metrics.UpdateAllocOperations(ts.allocates[op.k], keyChunks)
		ts.allocates[k] = keyChunks
		ts.metrics.UpdateWriteOperations(ts.writes[op.k], valueChunks)
		ts.writes[k] = valueChunks
	}
	ts.ops = append(ts.ops, op)
	ts.pendingChangedKeys[k] = maybe.Some(value)
	if ts.isUnchanged(ctx, k, value, true) {
		ts.metrics.UpdateAllocOperations(ts.allocates[op.k], 0)
		ts.metrics.UpdateWriteOperations(ts.writes[op.k], 0)
		delete(ts.allocates, k)
		delete(ts.writes, k)
		delete(ts.pendingChangedKeys, k)
	}
	return nil
}

// Remove deletes a key from [tstate]. If this action returns the
// value of [key] to the parent view, it reverts any pending changes.
func (ts *TStateView) Remove(ctx context.Context, key []byte) error {
	// Removing requires writing & deleting that key, so we pass state.Write
	if !ts.checkScope(ctx, key, state.Write) {
		return ErrInvalidKeyOrPermission
	}
	k := string(key)
	past, exists := ts.getValue(ctx, k)
	if !exists {
		// We do not update writes if the key does not exist.
		return nil
	}
	ts.ops = append(ts.ops, &op{
		t:             removeOp,
		k:             k,
		pastV:         past,
		pastAllocates: chunks(ts.allocates, k),
		pastWrites:    chunks(ts.writes, k),
	})
	if _, ok := ts.allocates[k]; ok {
		// If delete after allocating in the same view, it is
		// as if nothing happened.
		ts.metrics.UpdateAllocOperations(ts.allocates[k], 0)
		ts.metrics.UpdateWriteOperations(ts.writes[k], 0)
		delete(ts.allocates, k)
		delete(ts.writes, k)
		delete(ts.pendingChangedKeys, k)
		// We don't have access to the vm, so should probably provide a pointer to metrics
		// ts.metrics.AllocKeyOperations.Reset()
	} else {
		// If this is not a new allocation, we mark as an
		// explicit delete.
		ts.metrics.UpdateWriteOperations(ts.writes[k], 0)
		ts.writes[k] = 0
		ts.pendingChangedKeys[k] = maybe.Nothing[[]byte]()
	}
	if ts.isUnchanged(ctx, k, nil, false) {
		ts.metrics.UpdateAllocOperations(ts.allocates[k], 0)
		ts.metrics.UpdateWriteOperations(ts.writes[k], 0)
		delete(ts.allocates, k)
		delete(ts.writes, k)
		delete(ts.pendingChangedKeys, k)
	}
	return nil
}

// PendingChanges returns the number of changed keys (not ops).
func (ts *TStateView) PendingChanges() int {
	return len(ts.pendingChangedKeys)
}

// Commit adds all pending changes to the parent view.
func (ts *TStateView) Commit() {
	ts.ts.l.Lock()
	defer ts.ts.l.Unlock()

	for k, v := range ts.pendingChangedKeys {
		ts.ts.changedKeys[k] = v
	}
	ts.ts.ops += len(ts.ops)
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
