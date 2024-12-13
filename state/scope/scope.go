// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package scope

import (
	"context"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/state"
)

var (
	_ Scope = (*DefaultScope)(nil)
	_ Scope = (*SimulatedScope)(nil)
)

type Scope interface {
	Has(key []byte, perm state.Permissions) bool
	GetValue(ctx context.Context, key []byte) ([]byte, error)
	Len() int
}

type DefaultScope struct {
	keys    state.Keys
	storage map[string][]byte
}

func NewDefaultScope(keys state.Keys, storage map[string][]byte) *DefaultScope {
	return &DefaultScope{
		keys:    keys,
		storage: storage,
	}
}

func (d *DefaultScope) GetValue(_ context.Context, key []byte) ([]byte, error) {
	if v, has := d.storage[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

func (d *DefaultScope) Has(key []byte, perm state.Permissions) bool {
	return d.keys[string(key)].Has(perm)
}

func (d *DefaultScope) Len() int {
	return len(d.keys)
}

type SimulatedScope struct {
	keys state.Keys
	im   state.Immutable
}

func NewSimulatedScope(keys state.Keys, im state.Immutable) *SimulatedScope {
	return &SimulatedScope{
		keys: keys,
		im:   im,
	}
}

func (d *SimulatedScope) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	return d.im.GetValue(ctx, key)
}

func (d *SimulatedScope) Has(key []byte, perm state.Permissions) bool {
	d.keys.Add(string(key), perm)
	return true
}

func (d *SimulatedScope) Len() int {
	return len(d.keys)
}

func (d *SimulatedScope) StateKeys() state.Keys {
	return d.keys
}

// Flush clears the keys in the scope
func (d *SimulatedScope) Flush() {
	clear(d.keys)
}
