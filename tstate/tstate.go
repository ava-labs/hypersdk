// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type op struct {
	k string

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
	changedKeys map[string]maybe.Maybe[[]byte]
	fetchCache  map[string]*cacheItem // in case we evict and want to re-fetch

	// We don't differentiate between read and write scope.
	scope        set.Set[string] // stores a list of managed keys in the TState struct
	scopeStorage map[string][]byte

	// Ops is a record of all operations performed on [TState]. Tracking
	// operations allows for reverting state to a certain point-in-time.
	ops []*op

	// Store which keys are modified and how large their values were. Reset
	// whenever setting scope.
	canCreate         bool
	creations         map[string]uint16
	coldModifications map[string]uint16
	warmModifications map[string]uint16
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(changedSize int) *TState {
	return &TState{
		changedKeys: make(map[string]maybe.Maybe[[]byte], changedSize),

		fetchCache: map[string]*cacheItem{},

		ops: make([]*op, 0, changedSize),

		canCreate: true,
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

// Exists returns whether or not the associated [key] is present.
func (ts *TState) Exists(ctx context.Context, key []byte) (bool, bool, error) {
	if !ts.checkScope(ctx, key) {
		return false, false, ErrKeyNotSpecified
	}
	k := string(key)
	_, changed, exists := ts.getValue(ctx, k)
	return changed, exists, nil
}

func (ts *TState) getValue(_ context.Context, key string) ([]byte, bool, bool) {
	if v, ok := ts.changedKeys[key]; ok {
		if v.IsNothing() {
			return nil, true, false
		}
		return v.Value(), true, true
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
//
// If possible, this function should be avoided and state should be prefetched (much faster).
func (ts *TState) FetchAndSetScope(ctx context.Context, keys set.Set[string], im state.Immutable) error {
	ts.scopeStorage = map[string][]byte{}
	for key := range keys {
		if val, ok := ts.fetchCache[key]; ok {
			if val.Exists {
				ts.scopeStorage[key] = val.Value
			}
			continue
		}
		v, err := im.GetValue(ctx, []byte(key))
		if errors.Is(err, database.ErrNotFound) {
			ts.fetchCache[key] = &cacheItem{Exists: false}
			continue
		}
		if err != nil {
			return err
		}
		ts.fetchCache[key] = &cacheItem{Value: v, Exists: true}
		ts.scopeStorage[key] = v
	}
	ts.scope = keys
	ts.creations = map[string]uint16{}
	ts.coldModifications = map[string]uint16{}
	ts.warmModifications = map[string]uint16{}
	return nil
}

// SetReadScope sets the readscope of ts to [keys].
func (ts *TState) SetScope(_ context.Context, keys set.Set[string], storage map[string][]byte) {
	ts.scope = keys
	ts.scopeStorage = storage
	ts.creations = map[string]uint16{}
	ts.coldModifications = map[string]uint16{}
	ts.warmModifications = map[string]uint16{}
}

// DisableCreation causes [Insert] to return an error if
// it would create a new key. This can be useful for constraining
// what a transaction can do during block execution (to allow for
// cheaper fees).
//
// Note, creation defaults to true.
func (ts *TState) DisableCreation() {
	ts.canCreate = false
}

// EnableCreation removes the forcer error case in [Insert]
// if a new key is created.
//
// Note, creation defaults to true.
func (ts *TState) EnableCreation() {
	ts.canCreate = true
}

// checkScope returns whether [k] is in ts.readScope.
func (ts *TState) checkScope(_ context.Context, k []byte) bool {
	return ts.scope.Contains(string(k))
}

// Insert sets or updates ts.storage[key] to equal {value, false}.
//
// Any bytes passed into [Insert] will be consumed by [TState] and should
// not be modified/referenced after this call.
func (ts *TState) Insert(ctx context.Context, key []byte, value []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	if !keys.VerifyValue(key, value) {
		return ErrInvalidKeyValue
	}
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	var err error
	if exists {
		// If a key is already in [coldModifications], we should still
		// consider it a [coldModification] even if it is [changed].
		// This occurs when we modify a key for the second time in
		// a single transaction.
		//
		// If a key is not in [coldModifications] and it is [changed],
		// it was either created/modified in a different transaction
		// in the block or created in this transaction.
		if _, ok := ts.coldModifications[k]; ok || !changed {
			err = updateChunks(ts.coldModifications, k, value)
		} else {
			err = updateChunks(ts.warmModifications, k, value)
		}
	} else {
		if !ts.canCreate {
			err = ErrCreationDisabled
		} else {
			err = updateChunks(ts.creations, k, value)
		}
	}
	if err != nil {
		return err
	}
	ts.ops = append(ts.ops, &op{
		k:           k,
		pastExists:  exists,
		pastV:       past,
		pastChanged: changed,
	})
	ts.changedKeys[k] = maybe.Some(value)
	return nil
}

// Remove deletes a key-value pair from ts.storage.
func (ts *TState) Remove(ctx context.Context, key []byte) error {
	if !ts.checkScope(ctx, key) {
		return ErrKeyNotSpecified
	}
	k := string(key)
	past, changed, exists := ts.getValue(ctx, k)
	if !exists {
		// We do not update modificaations if the key does not exist.
		return nil
	}
	// If a key is already in [coldModifications], we should still
	// consider it a [coldModification] even if it is [changed].
	// This occurs when we modify a key for the second time in
	// a single transaction.
	//
	// If a key is not in [coldModifications] and it is [changed],
	// it was either created/modified in a different transaction
	// in the block or created in this transaction.
	var err error
	if _, ok := ts.coldModifications[k]; ok || !changed {
		err = updateChunks(ts.coldModifications, k, nil)
	} else {
		err = updateChunks(ts.warmModifications, k, nil)
	}
	if err != nil {
		return err
	}
	ts.ops = append(ts.ops, &op{
		k:           k,
		pastExists:  true,
		pastV:       past,
		pastChanged: changed,
	})
	ts.changedKeys[k] = maybe.Nothing[[]byte]()
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
		if !op.pastExists {
			ts.changedKeys[op.k] = maybe.Nothing[[]byte]()
		} else {
			ts.changedKeys[op.k] = maybe.Some(op.pastV)
		}
	}
	ts.ops = ts.ops[:restorePoint]
}

// CreateView creates a slice of [database.BatchOp] of all
// changes in [TState] that can be used to commit to [merkledb].
func (ts *TState) CreateView(
	ctx context.Context,
	view state.View,
	t trace.Tracer, //nolint:interfacer
) (merkledb.TrieView, error) {
	ctx, span := t.Start(
		ctx, "TState.CreateView",
		oteltrace.WithAttributes(
			attribute.Int("items", len(ts.changedKeys)),
		),
	)
	defer span.End()

	return view.NewView(ctx, merkledb.ViewChanges{MapOps: ts.changedKeys, ConsumeBytes: true})
}

// updateChunks sets the number of chunks associated with a key that will
// be returned in [KeyOperations].
func updateChunks(m map[string]uint16, key string, value []byte) error {
	chunks, ok := keys.NumChunks(value)
	if !ok {
		return ErrInvalidKeyValue
	}
	previousChunks, ok := m[key]
	if !ok || chunks > previousChunks {
		m[key] = chunks
	}
	return nil
}

// KeyOperations returns the number of operations performed since the scope
// was last set.
//
// If an operation is performed more than once during this time, the largest
// operation will be returned here (if 1 chunk then 2 chunks are written to a key,
// this function will return 2 chunks).
func (ts *TState) KeyOperations() (map[string]uint16, map[string]uint16, map[string]uint16) {
	return ts.creations, ts.coldModifications, ts.warmModifications
}
