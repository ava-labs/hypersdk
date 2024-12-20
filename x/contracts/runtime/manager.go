// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_                 ContractManager = &ContractStateManager{}
	ErrUnknownAccount                 = errors.New("unknown account")
	contractKeyBytes                  = []byte("contract")
)

const (
	// Global directory of contractIDs to contractBytes
	contractPrefix = 0x0

	// Prefix for all contract state spaces
	accountPrefix = 0x1
	// Associated data for an account, such as the contractID
	accountDataPrefix = 0x0
	// State space associated with an account
	accountStatePrefix = 0x1
)

// ContractStateManager is an out of the box implementation of the ContractManager interface.
// The contract state manager is responsible for managing all state keys associated with contracts.
type ContractStateManager struct {
	db state.Mutable
}

// NewContractStateManager returns a new ContractStateManager instance.
// [prefix] must be unique to ensures the contract's state space
// remains isolated from other state spaces in [db].
func NewContractStateManager(
	db state.Mutable,
	prefix []byte,
) *ContractStateManager {
	prefixedState := newPrefixStateMutable(prefix, db)

	return &ContractStateManager{
		db: prefixedState,
	}
}

// GetContractState returns a mutable state instance associated with [account].
func (p *ContractStateManager) GetContractState(account codec.Address) state.Mutable {
	return newAccountPrefixedMutable(account, p.db)
}

// GetAccountContract grabs the associated id with [account]. The ID is the key mapping to the contractbytes
// Errors if there is no found account or an error fetching
func (p *ContractStateManager) GetAccountContract(ctx context.Context, account codec.Address) (ContractID, error) {
	contractID, exists, err := p.getAccountContract(ctx, account)
	if err != nil {
		return ids.Empty[:], err
	}
	if !exists {
		return ids.Empty[:], ErrUnknownAccount
	}
	return contractID[:], nil
}

// [contractID] -> [contractBytes]
func (p *ContractStateManager) GetContractBytes(ctx context.Context, contractID ContractID) ([]byte, error) {
	// TODO: take fee out of balance?
	contractBytes, err := p.db.GetValue(ctx, contractKey(contractID))
	if err != nil {
		return []byte{}, ErrUnknownAccount
	}

	return contractBytes, nil
}

func (p *ContractStateManager) NewAccountWithContract(ctx context.Context, contractID ContractID, accountCreationData []byte) (codec.Address, error) {
	newID := sha256.Sum256(append(contractID, accountCreationData...))
	newAccount := codec.CreateAddress(0, newID)
	return newAccount, p.SetAccountContract(ctx, newAccount, contractID)
}

func (p *ContractStateManager) SetAccountContract(ctx context.Context, account codec.Address, contractID ContractID) error {
	return p.db.Insert(ctx, accountDataKey(account[:], contractKeyBytes), contractID)
}

// SetContractBytes stores [contract] at [contractID]
func (p *ContractStateManager) SetContractBytes(
	ctx context.Context,
	contractID ContractID,
	contract []byte,
) error {
	return p.db.Insert(ctx, contractKey(contractID[:]), contract)
}

func contractKey(key []byte) (k []byte) {
	k = make([]byte, 0, 1+len(key))
	k = append(k, contractPrefix)
	k = append(k, key...)
	return
}

// Creates a key an account balance key
func accountDataKey(account []byte, key []byte) (k []byte) {
	// accountPrefix + account + accountDataPrefix + key
	k = make([]byte, 0, 2+len(account)+len(key))
	k = append(k, accountPrefix)
	k = append(k, account...)
	k = append(k, accountDataPrefix)
	k = append(k, key...)
	return
}

func accountContractKey(account []byte) []byte {
	return accountDataKey(account, contractKeyBytes)
}

func (p *ContractStateManager) getAccountContract(ctx context.Context, account codec.Address) (ids.ID, bool, error) {
	v, err := p.db.GetValue(ctx, accountContractKey(account[:]))
	if errors.Is(err, database.ErrNotFound) {
		return ids.Empty, false, nil
	}
	if err != nil {
		return ids.Empty, false, err
	}
	return ids.ID(v[:ids.IDLen]), true, nil
}

// prefixed state
type prefixedStateMutable struct {
	inner  state.Mutable
	prefix []byte
}

func newPrefixStateMutable(prefix []byte, inner state.Mutable) *prefixedStateMutable {
	return &prefixedStateMutable{inner: inner, prefix: prefix}
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

// [accountPrefix] + [account] + [accountStatePrefix] = state space associated with a contract
func accountStateKey(key []byte) (k []byte) {
	k = make([]byte, 2+len(key))
	k[0] = accountPrefix
	copy(k[1:], key)
	k[len(k)-1] = accountStatePrefix
	return
}
