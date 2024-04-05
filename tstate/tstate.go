// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"slices"

	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/hypersdk/smap"
	"github.com/ava-labs/hypersdk/state"
	"golang.org/x/exp/maps"
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
func New(changedSize int) *TState {
	return &TState{
		viewKeys:    make([]state.Keys, 500_000), // set to max txs that could ever be in a single block
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
		if keys == nil {
			// Skip empty views (needed to avoid locking)
			continue
		}

		// Ensure we iterate deterministically
		keyArr := maps.Keys(keys)
		slices.Sort(keyArr)
		for _, key := range keyArr {
			if keys[key] == state.Read {
				continue
			}
			v, ok := ts.changedKeys.Get(key)
			if !ok {
				continue
			}
			if v.view != idx {
				// If we weren't the latest modification, skip
				continue
			}
			if err := f([]byte(key), v.v); err != nil {
				return err
			}
		}
	}
	return nil
}
