// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
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

var verifyInitialValues = false

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
	initialValue map[string]maybe.Maybe[[]byte]

	// Store which keys are modified and how large their values were.
	canAllocate bool
	allocates   map[string]uint16
	writes      map[string]uint16
}

func (ts *TStateView) Scope() state.Keys {
	return ts.scope
}
func (ts *TStateView) ScopeStorage() map[string][]byte {
	return ts.scopeStorage
}
func (ts *TStateView) InitialValue() map[string]maybe.Maybe[[]byte] {
	return ts.initialValue
}
func (ts *TStateView) PendingChangedKeys() map[string]maybe.Maybe[[]byte] {
	return ts.pendingChangedKeys
}

func marshalMap[T any](p *codec.Packer, m map[string]T, packValue func(*codec.Packer, T)) {
	keys := maps.Keys(m)
	slices.Sort(keys)
	p.PackInt(len(keys))
	for _, k := range keys {
		p.PackString(k)
		packValue(p, m[k])
	}
}

func unmarshalMap[T any](p *codec.Packer, unpackValue func(*codec.Packer) T) map[string]T {
	numKeys := p.UnpackInt(false)
	m := make(map[string]T, numKeys)
	for i := 0; i < numKeys; i++ {
		key := p.UnpackString(false)
		m[key] = unpackValue(p)
	}
	return m
}

func (ts *TStateView) Marshal(p *codec.Packer) {
	marshalMap(p, ts.scope, func(p *codec.Packer, v state.Permissions) {
		p.PackByte(byte(v))
	})
	marshalMap(p, ts.scopeStorage, func(p *codec.Packer, v []byte) {
		p.PackBytes(v)
	})
	marshalMap(p, ts.initialValue, func(p *codec.Packer, v maybe.Maybe[[]byte]) {
		p.PackBool(v.HasValue())
		p.PackBytes(v.Value())
	})
	marshalMap(p, ts.pendingChangedKeys, func(p *codec.Packer, v maybe.Maybe[[]byte]) {
		p.PackBool(v.HasValue())
		p.PackBytes(v.Value())
	})
}

func UnmarshalTStateView(p *codec.Packer) *TStateView {
	scope := unmarshalMap(p, func(p *codec.Packer) state.Permissions {
		return state.Permissions(p.UnpackByte())
	})
	scopeStorage := unmarshalMap(p, func(p *codec.Packer) []byte {
		var val []byte
		p.UnpackBytes(-1, false, &val)
		return val
	})
	initialValue := unmarshalMap(p, func(p *codec.Packer) maybe.Maybe[[]byte] {
		hasVal := p.UnpackBool()
		var val []byte
		p.UnpackBytes(-1, false, &val)
		if !hasVal {
			return maybe.Nothing[[]byte]()
		}
		return maybe.Some(val)
	})
	pendingChangedKeys := unmarshalMap(p, func(p *codec.Packer) maybe.Maybe[[]byte] {
		hasVal := p.UnpackBool()
		var val []byte
		p.UnpackBytes(-1, false, &val)
		if !hasVal {
			return maybe.Nothing[[]byte]()
		}
		return maybe.Some(val)
	})
	ts := New(0)
	for k, v := range initialValue {
		if _, exists := scopeStorage[k]; !exists {
			continue
		}
		ts.changedKeys[k] = v
	}
	return &TStateView{
		ts:                 ts,
		scope:              scope,
		initialValue:       initialValue,
		scopeStorage:       scopeStorage,
		pendingChangedKeys: pendingChangedKeys,

		ops:         make([]*op, 0, defaultOps),
		canAllocate: true, // default to allowing allocation
		allocates:   make(map[string]uint16, len(scope)),
		writes:      make(map[string]uint16, len(scope)),
	}
}

func (ts *TStateView) setInitialValues(ctx context.Context) {
	ts.initialValue = make(map[string]maybe.Maybe[[]byte], len(ts.scope))
	for k := range ts.scope {
		v, exists := ts.getValue(ctx, k)
		if exists {
			ts.initialValue[k] = maybe.Some(v)
		} else {
			ts.initialValue[k] = maybe.Nothing[[]byte]()
		}
	}
}

func (ts *TState) NewView(scope state.Keys, storage map[string][]byte) *TStateView {
	tsv := &TStateView{
		ts:                 ts,
		pendingChangedKeys: make(map[string]maybe.Maybe[[]byte], len(scope)),

		ops: make([]*op, 0, defaultOps),

		scope:        scope,
		scopeStorage: storage,

		canAllocate: true, // default to allowing allocation
		allocates:   make(map[string]uint16, len(scope)),
		writes:      make(map[string]uint16, len(scope)),
	}
	if verifyInitialValues {
		tsv.setInitialValues(context.Background())
	}
	return tsv
}

// Rollback restores the TState to the ts.op[restorePoint] operation.
func (ts *TStateView) Rollback(_ context.Context, restorePoint int) {
	for i := len(ts.ops) - 1; i >= restorePoint; i-- {
		op := ts.ops[i]

		switch op.t {
		case createOp:
			delete(ts.allocates, op.k)
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
			if op.pastAllocates != nil {
				// If we removed a newly created key, we need to restore it
				// to [allocates].
				//
				// The key should have already been removed from allocates,
				// so we don't need to explicitly delete it if [op.pastAllocates]
				// is nil.
				ts.allocates[op.k] = *op.pastAllocates
			}
			if op.pastWrites != nil {
				// If we removed a newly written key, we should revert
				// it back to its previous value.
				ts.writes[op.k] = *op.pastWrites
				ts.pendingChangedKeys[op.k] = maybe.Some(op.pastV)
			} else {
				// If we removed a key that already existed before the view,
				// we should remove it from tracking.
				//
				// This should never happen if [op.pastAllocates] != nil.
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
		return nil, fmt.Errorf(
			"%w: key %x not in scope or does not have read permission",
			ErrInvalidKeyOrPermission,
			key,
		)
	}
	k := string(key)
	v, exists := ts.getValue(ctx, k)
	if verifyInitialValues {
		if err := ts.verifyInitialValue(k, v, exists); err != nil {
			panic(err)
		}
	}
	if !exists {
		return nil, database.ErrNotFound
	}
	return v, nil
}

func (ts *TStateView) verifyInitialValue(key string, value []byte, exists bool) error {
	if _, ok := ts.pendingChangedKeys[key]; ok {
		return nil // if we have a pending change, we don't need to verify against initial value
	}
	if v, ok := ts.initialValue[key]; ok {
		if v.IsNothing() {
			if exists {
				return fmt.Errorf("initial value for key %x is not found", key)
			}
			return nil
		}
		if !bytes.Equal(v.Value(), value) {
			return fmt.Errorf("initial value for key %x is %x, not %x", key, v.Value(), value)
		}
	}
	return nil
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
		return fmt.Errorf(
			"%w: key %x not in scope or does not have write permission",
			ErrInvalidKeyOrPermission,
			key,
		)
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
		ts.writes[k] = valueChunks // set to latest value
	} else {
		// New entry requires Allocate
		// TODO: we assume any allocate is a write too, but we should
		// make this invariant more clear. Do we require Write,
		// Allocate|Write, and never Allocate alone?
		if !ts.checkScope(ctx, key, state.Allocate) {
			return fmt.Errorf(
				"%w: key %x not in scope or does not have allocate permission",
				ErrInvalidKeyOrPermission,
				key,
			)
		}
		if !ts.canAllocate {
			return ErrAllocationDisabled
		}
		op.t = createOp
		keyChunks, _ := keys.MaxChunks(key) // not possible to fail
		ts.allocates[k] = keyChunks
		ts.writes[k] = valueChunks
	}
	ts.ops = append(ts.ops, op)
	ts.pendingChangedKeys[k] = maybe.Some(value)
	if ts.isUnchanged(ctx, k, value, true) {
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
		return fmt.Errorf(
			"%w: key %x not in scope or does not have write permission",
			ErrInvalidKeyOrPermission,
			key,
		)
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
		delete(ts.allocates, k)
		delete(ts.writes, k)
		delete(ts.pendingChangedKeys, k)
	} else {
		// If this is not a new allocation, we mark as an
		// explicit delete.
		ts.writes[k] = 0
		ts.pendingChangedKeys[k] = maybe.Nothing[[]byte]()
	}
	if ts.isUnchanged(ctx, k, nil, false) {
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

func (ts *TStateView) LogChangedKeys(log logging.Logger, prefix string, height uint64, txID ids.ID) {
	for k, v := range ts.pendingChangedKeys {
		if v.IsNothing() {
			log.Info(
				"TStateView deleted key",
				zap.String("key", common.Bytes2Hex([]byte(k))),
				zap.String("prefix", prefix),
				zap.Uint64("height", height),
				zap.Stringer("txID", txID),
			)
		} else {
			log.Info(
				"TStateView changed key",
				zap.String("key", common.Bytes2Hex([]byte(k))),
				zap.String("value", common.Bytes2Hex(v.Value())),
				zap.String("prefix", prefix),
				zap.Uint64("height", height),
				zap.Stringer("txID", txID),
			)
		}
	}
}
