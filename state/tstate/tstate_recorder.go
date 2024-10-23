// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/ava-labs/hypersdk/state"
)

type TStateRecorder struct {
	stateView *TStateView
}

func NewRecorder(immutableState state.Immutable) *TStateRecorder {
	sr := &TStateRecorder{}
	sr.stateView = sr.newRecorderView(immutableState)
	return sr
}

func (sr *TStateRecorder) Insert(ctx context.Context, key []byte, value []byte) error {
	return sr.stateView.Insert(ctx, key, value)
}

func (sr *TStateRecorder) Remove(ctx context.Context, key []byte) error {
	return sr.stateView.Remove(ctx, key)
}

func (sr *TStateRecorder) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return sr.stateView.GetValue(ctx, key)
}

// GetStateKeys returns the keys that have been touched along with their respective permissions.
func (sr *TStateRecorder) GetStateKeys() state.Keys {
	return sr.stateView.GetStateKeys()
}
