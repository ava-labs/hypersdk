package storage

import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

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

func (p *ProgramStateManager) GetAccountProgram(ctx context.Context, account codec.Address) (ids.ID, error) {
	result, err := p.GetValue(ctx, AccountProgramKey(account))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(result), nil
}

func (p *ProgramStateManager) GetProgramBytes(ctx context.Context, programID ids.ID) ([]byte, error) {
	return p.GetValue(ctx, ProgramsKey(programID))
}

func (p *ProgramStateManager) NewAccountWithProgram(ctx context.Context, programID ids.ID, accountCreationData []byte) (codec.Address, error) {
	newAddress := GetAddressForDeploy(0, accountCreationData)
	_, err := p.GetValue(ctx, AccountProgramKey(newAddress))
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return codec.EmptyAddress, err
	} else if err == nil {
		return codec.EmptyAddress, errors.New("account already exists")
	}

	return newAddress, p.SetAccountProgram(ctx, newAddress, programID)
}

func (p *ProgramStateManager) SetAccountProgram(ctx context.Context, account codec.Address, programID ids.ID) error {
	return p.Insert(ctx, AccountProgramKey(account), programID[:])
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
