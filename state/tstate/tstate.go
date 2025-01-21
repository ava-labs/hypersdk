// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/utils/maybe"
)

// TState defines a struct for storing temporary state.
type TState struct {
	l           sync.RWMutex
	ops         int
	changedKeys map[string]maybe.Maybe[[]byte]
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(changedSize int) *TState {
	return &TState{
		changedKeys: make(map[string]maybe.Maybe[[]byte], changedSize),
	}
}

func (ts *TState) getChangedValue(_ context.Context, key string) ([]byte, bool, bool) {
	ts.l.RLock()
	defer ts.l.RUnlock()

	if v, ok := ts.changedKeys[key]; ok {
		if v.IsNothing() {
			return nil, true, false
		}
		return v.Value(), true, true
	}
	return nil, false, false
}

func (ts *TState) PendingChanges() int {
	ts.l.RLock()
	defer ts.l.RUnlock()

	return len(ts.changedKeys)
}

// OpIndex returns the number of operations done on ts.
func (ts *TState) OpIndex() int {
	ts.l.RLock()
	defer ts.l.RUnlock()

	return ts.ops
}

func (ts *TState) ChangedKeys() map[string]maybe.Maybe[[]byte] {
	ts.l.RLock()
	defer ts.l.RUnlock()

	return ts.changedKeys
}
