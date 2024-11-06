// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import "github.com/ava-labs/hypersdk/state"

type TStateRecorder struct {
	*TStateView

	recorder ScopeRecorder
}

type ScopeRecorder map[string]state.Permissions

func (s ScopeRecorder) Has(key string, perm state.Permissions) bool {
	s[key] |= perm
	return true
}

func (s ScopeRecorder) Len() int { return len(s) }

func NewRecorder(storage state.Immutable) *TStateRecorder {
	ts := New(0)
	scope := make(ScopeRecorder)
	tStateView := ts.newView(scope, storage)

	return &TStateRecorder{
		TStateView: tStateView,
		recorder:   scope,
	}
}

func (ts *TStateRecorder) GetStateKeys() state.Keys {
	return state.Keys(ts.recorder)
}
