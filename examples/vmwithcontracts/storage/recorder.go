// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/state"
)

type Recorder struct {
	State         state.Immutable
	changedValues map[string][]byte
	ReadState     set.Set[string]
	WriteState    set.Set[string]
}

func NewRecorder(db state.Immutable) *Recorder {
	return &Recorder{State: db, changedValues: map[string][]byte{}}
}

func (r *Recorder) Insert(_ context.Context, key []byte, value []byte) error {
	stringKey := string(key)
	r.WriteState.Add(stringKey)
	r.changedValues[stringKey] = value
	return nil
}

func (r *Recorder) Remove(_ context.Context, key []byte) error {
	stringKey := string(key)
	r.WriteState.Add(stringKey)
	r.changedValues[stringKey] = nil
	return nil
}

func (r *Recorder) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	stringKey := string(key)
	r.ReadState.Add(stringKey)
	if value, ok := r.changedValues[stringKey]; ok {
		if value == nil {
			return nil, database.ErrNotFound
		}
		return value, nil
	}
	return r.State.GetValue(ctx, key)
}

func (r *Recorder) GetStateKeys() state.Keys {
	result := state.Keys{}
	for key := range r.ReadState {
		result.Add(key, state.Read)
	}
	for key := range r.WriteState {
		if _, err := r.State.GetValue(context.Background(), []byte(key)); err != nil && errors.Is(err, database.ErrNotFound) {
			if r.changedValues[key] == nil {
				// not a real write since the key was not already present and is being deleted
				continue
			}
			// wasn't found so needs to be allocated
			result.Add(key, state.Allocate)
		}
		result.Add(key, state.Write)
	}
	return result
}
