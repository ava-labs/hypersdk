// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/consts"
)

var (
	_ Scope = (*DefaultScope)(nil)
	_ Scope = (*SimulatedScope)(nil)
)

var ErrValueTooShortToBeSuffixed = errors.New("value is too short to be suffixed")

type Scope interface {
	Has(key []byte, perm Permissions) bool
	GetValue(ctx context.Context, key []byte) ([]byte, error)
	Len() int
}

type DefaultScope struct {
	keys    Keys
	storage map[string][]byte
}

func NewDefaultScope(keys Keys, storage map[string][]byte) *DefaultScope {
	return &DefaultScope{
		keys:    keys,
		storage: storage,
	}
}

// Invariant: storage is a set of KV-pairs where the values are suffixed with
// the block height at which they were last touched
func NewUnsuffixedDefaultScope(keys Keys, storage map[string][]byte) (*DefaultScope, error) {
	st := make(map[string][]byte, len(storage))

	for k, v := range storage {
		if len(v) < consts.Uint64Len {
			return nil, ErrValueTooShortToBeSuffixed
		}
		st[k] = v[:len(v)-consts.Uint64Len]
	}

	return &DefaultScope{
		keys:    keys,
		storage: st,
	}, nil
}

func (d *DefaultScope) GetValue(_ context.Context, key []byte) ([]byte, error) {
	if v, has := d.storage[string(key)]; has {
		return v, nil
	}
	return nil, database.ErrNotFound
}

func (d *DefaultScope) Has(key []byte, perm Permissions) bool {
	return d.keys[string(key)].Has(perm)
}

func (d *DefaultScope) Len() int {
	return len(d.keys)
}

// TODO: is this even scope?
type SimulatedScope struct {
	keys Keys
	im   Immutable
}

func NewSimulatedScope(keys Keys, im Immutable) *SimulatedScope {
	return &SimulatedScope{
		keys: keys,
		im:   im,
	}
}

func (d *SimulatedScope) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	return d.im.GetValue(ctx, key)
}

func (d *SimulatedScope) Has(key []byte, perm Permissions) bool {
	d.keys.Add(string(key), perm)
	return true
}

func (d *SimulatedScope) Len() int {
	return len(d.keys)
}

func (d *SimulatedScope) StateKeys() Keys {
	return d.keys
}

// Flush clears the keys in the scope
func (d *SimulatedScope) Flush() {
	clear(d.keys)
}
