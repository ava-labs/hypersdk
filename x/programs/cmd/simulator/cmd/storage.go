// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

const (
	programPrefix = 0x0

	accountPrefix      = 0x1
	accountDataPrefix  = 0x0
	accountStatePrefix = 0x1

	addressStoragePrefix = 0x3
)

type programStateManager struct {
	state.Mutable
}

func (s *programStateManager) GetAccountProgram(ctx context.Context, account codec.Address) (ids.ID, error) {
	programID, exists, err := getAccountProgram(ctx, s, account)
	if err != nil {
		return ids.Empty, err
	}
	if !exists {
		return ids.Empty, errors.New("unknown account")
	}

	return programID, nil
}

func (s *programStateManager) GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error) {
	// TODO: take fee out of balance?
	programBytes, exists, err := getProgram(ctx, s, programID)
	if err != nil {
		return []byte{}, err
	}
	if !exists {
		return []byte{}, errors.New("unknown program")
	}

	return programBytes, nil
}

func (s *programStateManager) NewAccountWithProgram(ctx context.Context, programID ids.ID, accountCreationData []byte) (codec.Address, error) {
	return deployProgram(ctx, s, programID, accountCreationData)
}

func (s *programStateManager) SetAccountProgram(ctx context.Context, account codec.Address, programID ids.ID) error {
	return setAccountProgram(ctx, s, account, programID)
}

func (s *programStateManager) GetProgramState(account codec.Address) state.Mutable {
	return newAccountPrefixedMutable(account, s)
}

type prefixedStateMutable struct {
	inner  state.Mutable
	prefix []byte
}

func (s *prefixedStateMutable) prefixKey(key []byte) (k []byte) {
	k = make([]byte, len(s.prefix)+len(key))
	copy(k, s.prefix)
	copy(k[len(s.prefix):], key)
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
	return &prefixedStateMutable{inner: mutable, prefix: accountStateKey(account[:])}
}

//
// Program
//

func accountStateKey(key []byte) (k []byte) {
	k = make([]byte, 2+len(key))
	k[0] = accountPrefix
	copy(k[1:], key)
	k[len(k)-1] = accountStatePrefix
	return
}

func accountDataKey(key []byte) (k []byte) {
	k = make([]byte, 2+len(key))
	k[0] = accountPrefix
	copy(k[1:], key)
	k[len(k)-1] = accountDataPrefix
	return
}

func programKey(key []byte) (k []byte) {
	k = make([]byte, 1+len(key))
	k[0] = programPrefix
	copy(k[1:], key)
	return
}

// [programID] -> [programBytes]
func getAccountProgram(
	ctx context.Context,
	db state.Immutable,
	account codec.Address,
) (
	ids.ID,
	bool, // exists
	error,
) {
	v, err := db.GetValue(ctx, accountDataKey(account[:]))
	if errors.Is(err, database.ErrNotFound) {
		return ids.Empty, false, nil
	}
	if err != nil {
		return ids.Empty, false, err
	}
	return ids.ID(v[:32]), true, nil
}

func setAccountProgram(
	ctx context.Context,
	mu state.Mutable,
	account codec.Address,
	programID ids.ID,
) error {
	return mu.Insert(ctx, accountDataKey(account[:]), programID[:])
}

// [programID] -> [programBytes]
func getProgram(
	ctx context.Context,
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	v, err := db.GetValue(ctx, programKey(programID[:]))
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// setProgram stores [program] at [programID]
func setProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	program []byte,
) error {
	return mu.Insert(ctx, programKey(programID[:]), program)
}

func deployProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	accountCreationData []byte,
) (codec.Address, error) {
	newID := sha256.Sum256(append(programID[:], accountCreationData...))
	newAccount := codec.CreateAddress(0, newID)
	return newAccount, setAccountProgram(ctx, mu, newAccount, programID)
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
