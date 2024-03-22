// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
)

// TState defines a struct for storing temporary state.
type TState struct {
	changedKeys sync.Map
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(changedSize int) *TState {
	return &TState{}
}

func (ts *TState) getChangedValue(_ context.Context, key string) ([]byte, bool, bool) {
	if rv, ok := ts.changedKeys.Load(key); ok {
		v := rv.(maybe.Maybe[[]byte])
		if v.IsNothing() {
			return nil, true, false
		}
		return v.Value(), true, true
	}
	return nil, false, false
}

// Insert should only be called if you know what you are doing (updates
// here may not be reflected in get calls in tstate views and/or may overwrite
// identical keys on disk).
func (ts *TState) Insert(ctx context.Context, key, value []byte) error {
	if !keys.VerifyValue(key, value) {
		return ErrInvalidKeyValue
	}

	ts.changedKeys.Store(string(key), maybe.Some(value)) // we don't care if key is equivalent to key on-disk or in `changedKeys`
	return nil
}

// ExportMerkleDBView creates a slice of [database.BatchOp] of all
// changes in [TState] that can be used to commit to [merkledb].
func (ts *TState) ExportMerkleDBView(
	ctx context.Context,
	t trace.Tracer, //nolint:interfacer
	view state.View,
) (merkledb.View, int, error) {
	ctx, span := t.Start(ctx, "TState.ExportMerkleDBView")
	defer span.End()

	// Construct DB changes
	//
	// We are willing to accept the penalty of a full iteration here
	// to have better performance while updating TState.
	mapOps := make(map[string]maybe.Maybe[[]byte], 16_384) // TODO: make a config
	ts.changedKeys.Range(func(key, value interface{}) bool {
		mapOps[key.(string)] = value.(maybe.Maybe[[]byte])
		return true
	})
	nv, err := view.NewView(ctx, merkledb.ViewChanges{MapOps: mapOps, ConsumeBytes: true})
	return nv, len(mapOps), err
}
