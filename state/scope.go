// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

var (
	_ Scope = (*DefaultScope)(nil)
	_ Scope = (*SimulatedScope)(nil)
)

type Scope interface {
	Has(key []byte, perm Permissions) bool
}

type DefaultScope struct {
	keys Keys
}

func NewDefaultScope(keys Keys) *DefaultScope {
	return &DefaultScope{
		keys: keys,
	}
}

func (d *DefaultScope) Has(key []byte, perm Permissions) bool {
	return d.keys[string(key)].Has(perm)
}

type SimulatedScope struct {
	keys Keys
}

func NewSimulatedScope(keys Keys) *SimulatedScope {
	return &SimulatedScope{
		keys: keys,
	}
}

func (d *SimulatedScope) Has(key []byte, perm Permissions) bool {
	d.keys.Add(string(key), perm)
	return true
}

func (d *SimulatedScope) StateKeys() Keys {
	return d.keys
}

// Flush clears the keys in the scope
func (d *SimulatedScope) Flush() {
	clear(d.keys)
}
