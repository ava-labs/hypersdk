// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
)

// ImmutableStorage implements [state.Immutable] by wrapping a key-value map
type ImmutableStorage map[string][]byte

func (i ImmutableStorage) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	if v, has := i[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

type MutableStorage map[string][]byte

func (m MutableStorage) GetValue(_ context.Context, key []byte) (value []byte, err error) {
	if v, has := m[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

func (m MutableStorage) Insert(_ context.Context, key []byte, value []byte) error {
	m[string(key)] = value
	return nil
}

func (m MutableStorage) Remove(_ context.Context, key []byte) error {
	delete(m, string(key))
	return nil
}
