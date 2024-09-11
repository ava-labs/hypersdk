// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

// [accountStatePrefix] + [account]
func accountStateKey(account codec.Address) (k []byte) {
	k = make([]byte, 2+codec.AddressLen)
	k[0] = accountsPrefix
	copy(k[1:], account[:])
	k[len(k)-1] = accountStatePrefix
	return
}

func AccountProgramKey(account codec.Address) (k []byte) {
	k = make([]byte, 2+codec.AddressLen)
	k[0] = accountsPrefix
	copy(k[1:], account[:])
	k[len(k)-1] = accountProgramPrefix
	return
}

func ProgramsKey(id []byte) (k []byte) {
	k = make([]byte, 1+len(id))
	k[0] = programsPrefix
	copy(k[1:], id)
	return
}

func StoreProgram(
	ctx context.Context,
	mu state.Mutable,
	programBytes []byte,
) ([]byte, error) {
	programID := ids.ID(sha256.Sum256(programBytes))
	key, _ := keys.Encode(ProgramsKey(programID[:]), len(programBytes))
	return key, mu.Insert(ctx, key, programBytes)
}

func GetAddressForDeploy(typeID uint8, creationData []byte) codec.Address {
	digest := sha256.Sum256(creationData)
	return codec.CreateAddress(typeID, digest)
}

var _ runtime.StateManager = (*ProgramStateManager)(nil)

type ProgramStateManager struct {
	state.Mutable
}

func (p *ProgramStateManager) GetBalance(ctx context.Context, address codec.Address) (uint64, error) {
	_, balance, _, err := getBalance(ctx, p, address)
	return balance, err
}

func (p *ProgramStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
	if err := SubBalance(ctx, p, from, amount); err != nil {
		return err
	}
	return AddBalance(ctx, p, to, amount, true)
}

func (p *ProgramStateManager) GetProgramState(address codec.Address) state.Mutable {
	return &prefixedStateMutable{prefix: accountStateKey(address), inner: p}
}

func (p *ProgramStateManager) GetAccountProgram(ctx context.Context, account codec.Address) (runtime.ProgramID, error) {
	key, _ := keys.Encode(AccountProgramKey(account), 36)
	result, err := p.GetValue(ctx, key)
	if err != nil {
		return ids.Empty[:], err
	}
	return result, nil
}

func (p *ProgramStateManager) GetProgramBytes(ctx context.Context, programID runtime.ProgramID) ([]byte, error) {
	return p.GetValue(ctx, programID)
}

func (p *ProgramStateManager) NewAccountWithProgram(ctx context.Context, programID runtime.ProgramID, accountCreationData []byte) (codec.Address, error) {
	newAddress := GetAddressForDeploy(0, accountCreationData)
	key, _ := keys.Encode(AccountProgramKey(newAddress), 36)
	_, err := p.GetValue(ctx, key)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return codec.EmptyAddress, err
	} else if err == nil {
		return codec.EmptyAddress, errors.New("account already exists")
	}

	return newAddress, p.SetAccountProgram(ctx, newAddress, programID)
}

func (p *ProgramStateManager) SetAccountProgram(ctx context.Context, account codec.Address, programID runtime.ProgramID) error {
	key, _ := keys.Encode(AccountProgramKey(account), 36)
	return p.Insert(ctx, key, programID)
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
