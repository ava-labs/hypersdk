// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/consts"
)

var (
	_ Scope = (*TieredScope)(nil)
	_ Scope = (*SimulatedKeyScope)(nil)
	_ Scope = (*DefaultScope)(nil)
)

var (
	ErrValueTooShort        = errors.New("value is too short to contain suffix")
	ErrOptimisticReadFailed = errors.New("optimistic read failed")
)

type Scope interface {
	Has(key []byte, perm Permissions) bool
	GetValue(ctx context.Context, key string) ([]byte, error)
	Len() int
	GetStateKeys() Keys
}

// Meant to be used inside TStateView
type TieredScope struct {
	keys            Keys
	localState      map[string][]byte
	persistentState Immutable
	blockHeight     uint64
	epsilon         uint64
}

// Invariant: the values in ims are suffixed with the last touched block height
// Furthermore, the KV-pairs in ims have ReadFromStorage permissions
func NewTieredScope(
	keys Keys,
	ims map[string][]byte,
	im Immutable,
	blockHeight uint64,
	epsilon uint64,
) (*TieredScope, error) {
	st := make(map[string][]byte, len(ims))
	for k, v := range ims {
		if len(v) < consts.Uint64Len {
			return nil, ErrValueTooShort
		}
		// Remove suffix
		st[k] = v[:len(v)-consts.Uint64Len]
	}
	return &TieredScope{
		keys:            keys,
		localState:      st,
		persistentState: im,
		blockHeight:     blockHeight,
		epsilon:         epsilon,
	}, nil
}

// If not in in-memory state, check persistent state
// If in persistent state, but is young, return the value
// Otherwise, if old, return the value if it has the correct permissions and
// error otherwise
func (s *TieredScope) GetValue(ctx context.Context, key string) ([]byte, error) {
	v, ok := s.localState[key]
	if ok {
		// Value was previously found in in-memory state or was a paid persistent read
		return v, nil
	}
	// Since value not available locally, we now optimistically read from
	// persistent state
	// Assumption: perm(key) == ReadFromMemory
	v, err := s.persistentState.GetValue(ctx, []byte(key))
	if err != nil && err != database.ErrNotFound {
		return nil, err
	}
	if err == database.ErrNotFound {
		if !s.keys[key].Has(ReadFromStorage) {
			// Value was not found in persistent state, and we didn't have
			// ReadFromStorage permissions. Therefore, we error
			return nil, ErrOptimisticReadFailed
		}
		// Value was not found in persistent state, but we had ReadFromStorage
		// permissions. Therefore, we delegate this back to the caller to handle
		return v, database.ErrNotFound
	}
	// Value was found in persistent state
	// Perform epsilon check
	lastTouched := binary.BigEndian.Uint64(v[len(v)-consts.Uint64Len:])
	if s.blockHeight-lastTouched > s.epsilon {
		// Value is old, and we don't have ReadFromStorage permissions
		return nil, ErrOptimisticReadFailed
	}
	// Value is young, and we can return it
	// We can also cache it in in-memory state
	// NOTE: we need to remove the suffix
	v = v[:len(v)-consts.Uint64Len]
	s.localState[key] = v
	return v, nil
}

func (s *TieredScope) Has(key []byte, perm Permissions) bool {
	return s.keys[string(key)].Has(perm)
}

func (s *TieredScope) Len() int {
	return len(s.keys)
}

func (s *TieredScope) GetStateKeys() Keys {
	return s.keys
}

type SimulatedKeyScope struct {
	keys Keys
	im   Immutable
}

func NewSimulatedKeyScope(keys Keys, im Immutable) *SimulatedKeyScope {
	return &SimulatedKeyScope{
		keys: keys,
		im:   im,
	}
}

func (s *SimulatedKeyScope) GetValue(ctx context.Context, key string) ([]byte, error) {
	return s.im.GetValue(ctx, []byte(key))
}

func (s *SimulatedKeyScope) Has(key []byte, perm Permissions) bool {
	s.keys.Add(string(key), perm)
	return true
}

func (s *SimulatedKeyScope) Len() int {
	return len(s.keys)
}

func (s *SimulatedKeyScope) GetStateKeys() Keys {
	return s.keys
}

type DefaultScope struct {
	keys Keys
	im   Immutable
}

func NewDefaultScope(keys Keys, im Immutable) *DefaultScope {
	return &DefaultScope{
		keys: keys,
		im:   im,
	}
}

func (s *DefaultScope) GetValue(ctx context.Context, key string) ([]byte, error) {
	return s.im.GetValue(ctx, []byte(key))
}

func (s *DefaultScope) Has(key []byte, perm Permissions) bool {
	return s.keys[string(key)].Has(perm)
}

func (s *DefaultScope) Len() int {
	return len(s.keys)
}

func (s *DefaultScope) GetStateKeys() Keys {
	return s.keys
}
