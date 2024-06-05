// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/state"
)

const (
	programPrefix        = 0x0
	programStatePrefix   = 0x1
	addressStoragePrefix = 0x2
)

type programStateLoader struct {
	inner state.Mutable
}

func (p programStateLoader) GetProgramState(account codec.Address) state.Mutable {
	return newAccountPrefixedMutable(account, p.inner)
}

type prefixedStateMutable struct {
	inner  state.Mutable
	prefix []byte
}

func (s *prefixedStateMutable) prefixKey(key []byte) (k []byte) {
	k = make([]byte, 1+len(s.prefix))
	copy(k, s.prefix)
	copy(k[len(s.prefix):], key[:])
	return
}

func (s *prefixedStateMutable) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return s.inner.GetValue(ctx, s.prefixKey(key))
}

func (s *prefixedStateMutable) Insert(ctx context.Context, key []byte, value []byte) error {
	return s.inner.Insert(ctx, s.prefixKey(key), value)
}

func (s *prefixedStateMutable) Remove(ctx context.Context, key []byte) error {
	return s.inner.Remove(ctx, s.prefixKey(key))
}

func newAccountPrefixedMutable(account codec.Address, mutable state.Mutable) state.Mutable {
	return &prefixedStateMutable{inner: mutable, prefix: programStateKey(account[:])}
}

//
// Program
//

func programStateKey(key []byte) (k []byte) {
	k = make([]byte, 1+len(key))
	k[0] = programStatePrefix
	copy(k[1:], key[:])
	return
}

func programKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+ids.IDLen)
	k[0] = programPrefix
	copy(k[1:], id[:])
	return
}

// [programID] -> [programBytes]
func GetProgram(
	ctx context.Context,
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	k := programKey(programID)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// setProgram stores [program] at [programID]
func SetProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	program []byte,
) error {
	return mu.Insert(ctx, programKey(programID), program)
}

// gets the public key mapped to the given name.
func GetPublicKey(ctx context.Context, db state.Immutable, name string) (ed25519.PublicKey, bool, error) {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = addressStoragePrefix
	copy(k[1:], name)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPublicKey, false, nil
	}
	if err != nil {
		return ed25519.EmptyPublicKey, false, err
	}
	return ed25519.PublicKey(v), true, nil
}

func SetKey(ctx context.Context, db state.Mutable, privateKey ed25519.PrivateKey, name string) error {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = addressStoragePrefix
	copy(k[1:], name)
	return db.Insert(ctx, k, privateKey[:])
}
