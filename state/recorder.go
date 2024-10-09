// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
)

// The Recorder struct wraps an [Immutable] state object and tracks the permissions used
// against the various keys. The Recorder struct implements the [Mutable] interface, allowing
// it to act as a direct replacement for a database view.
// The Recorder struct maintains the same semantics as TStateView in regards to the various
// required access permissions.
type Recorder struct {
	// State is the underlying [Immutable] object
	state     Immutable
	stateKeys map[string][]byte

	changedValues map[string][]byte
	keys          Keys
}

func NewRecorder(db Immutable) *Recorder {
	return &Recorder{state: db, changedValues: map[string][]byte{}, stateKeys: map[string][]byte{}, keys: Keys{}}
}

func (r *Recorder) checkState(ctx context.Context, key []byte) ([]byte, error) {
	if val, has := r.stateKeys[string(key)]; has {
		return val, nil
	}
	value, err := r.state.GetValue(ctx, key)
	if err == nil {
		// no error, key found.
		r.stateKeys[string(key)] = value
		return value, nil
	}

	if errors.Is(err, database.ErrNotFound) {
		r.stateKeys[string(key)] = nil
		err = nil
	}
	return nil, err
}

func (r *Recorder) Insert(ctx context.Context, key []byte, value []byte) error {
	stringKey := string(key)

	stateKeyVal, err := r.checkState(ctx, key)
	if err != nil {
		return err
	}

	if stateKeyVal != nil {
		// underlying storage already has that key.
		r.keys[stringKey] |= Write
	} else {
		// underlying storage doesn't have that key.
		r.keys[stringKey] |= Allocate | Write
	}

	// save the updated value.
	r.changedValues[stringKey] = value
	return nil
}

func (r *Recorder) Remove(_ context.Context, key []byte) error {
	stringKey := string(key)
	r.keys[stringKey] |= Write
	r.changedValues[stringKey] = nil
	return nil
}

func (r *Recorder) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	stringKey := string(key)

	var stateKeyVal []byte

	if stateKeyVal, err = r.checkState(ctx, key); err != nil {
		return nil, err
	}
	r.keys[stringKey] |= Read
	if value, ok := r.changedValues[stringKey]; ok {
		if value == nil { // value was removed.
			return nil, database.ErrNotFound
		}
		return value, nil
	}
	if stateKeyVal == nil { // no such key exist.
		return nil, database.ErrNotFound
	}
	return stateKeyVal, nil
}

func (r *Recorder) GetStateKeys() Keys {
	return r.keys
}
