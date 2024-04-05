// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/hypersdk/smap"
	"github.com/ava-labs/hypersdk/state"
)

type change struct {
	view int
	v    maybe.Maybe[[]byte]
}

// TState defines a struct for storing temporary state.
type TState struct {
	viewKeys    []state.Keys
	changedKeys *smap.SMap[*change]
}

// New returns a new instance of TState.
//
// [changedSize] is an estimate of the number of keys that will be changed and is
// used to optimize the creation of [ExportMerkleDBView].
func New(capacity int, changedSize int) *TState {
	return &TState{
		viewKeys:    make([]state.Keys, capacity),
		changedKeys: smap.New[*change](changedSize),
	}
}

func (ts *TState) getChangedValue(_ context.Context, key string) ([]byte, bool, bool) {
	if v, ok := ts.changedKeys.Get(key); ok {
		if v.v.IsNothing() {
			return nil, true, false
		}
		return v.v.Value(), true, true
	}
	return nil, false, false
}

// Iterate over changes in deterministic order
//
// Iterate should only be called once tstate is done being modified.
func (ts *TState) Iterate(f func([]byte, maybe.Maybe[[]byte]) error) error {
	for idx, keys := range ts.viewKeys {
		for key, perms := range keys {
			if perms == state.Read {
				continue
			}
			v, ok := ts.changedKeys.Get(key)
			if !ok {
				continue
			}
			if v.view != idx {
				continue
			}
			if err := f([]byte(key), v.v); err != nil {
				return err
			}
		}
	}
	return nil
}
