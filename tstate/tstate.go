// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/smap"
)

// TState defines a struct for storing temporary state.
type TState struct {
	changedKeys *smap.SMap[maybe.Maybe[[]byte]]
}

// New returns a new instance of TState.
//
// [changedSize] is an estimate of the number of keys that will be changed and is
// used to optimize the creation of [ExportMerkleDBView].
func New(changedSize int) *TState {
	return &TState{changedKeys: smap.New[maybe.Maybe[[]byte]](changedSize)}
}

func (ts *TState) getChangedValue(_ context.Context, key string) ([]byte, bool, bool) {
	if v, ok := ts.changedKeys.Get(key); ok {
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

	ts.changedKeys.Set(string(key), maybe.Some(value)) // we don't care if key is equivalent to key on-disk or in `changedKeys`
	return nil
}

func (ts *TState) Keys() *smap.SMap[maybe.Maybe[[]byte]] {
	return ts.changedKeys
}
