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
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

// [accountStatePrefix] + [account]
func accountStateKey(account codec.Address) (k []byte) {
	k = make([]byte, 2+codec.AddressLen)
	k[0] = accountsPrefix
	copy(k[1:], account[:])
	k[len(k)-1] = accountStatePrefix
	return
}

func AccountContractKey(account codec.Address) (k []byte) {
	k = make([]byte, 2+codec.AddressLen)
	k[0] = accountsPrefix
	copy(k[1:], account[:])
	k[len(k)-1] = accountContractPrefix
	return
}

func ContractsKey(id []byte) (k []byte) {
	k = make([]byte, 1+len(id))
	k[0] = contractsPrefix
	copy(k[1:], id)
	return
}

func StoreContract(
	ctx context.Context,
	mu state.Mutable,
	contractBytes []byte,
) ([]byte, error) {
	contractID := ids.ID(sha256.Sum256(contractBytes))
	key, _ := keys.Encode(ContractsKey(contractID[:]), len(contractBytes))
	return key, mu.Insert(ctx, key, contractBytes)
}

func GetAddressForDeploy(typeID uint8, creationData []byte) codec.Address {
	digest := sha256.Sum256(creationData)
	return codec.CreateAddress(typeID, digest)
}

var _ runtime.StateManager = (*ContractStateManager)(nil)

type ContractStateManager struct {
	state.Mutable
}

func (p *ContractStateManager) GetBalance(ctx context.Context, address codec.Address) (uint64, error) {
	_, balance, _, err := getBalance(ctx, p, address)
	return balance, err
}

func (p *ContractStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
	if _, err := SubBalance(ctx, p, from, amount); err != nil {
		return err
	}
	_, err := AddBalance(ctx, p, to, amount, true)
	return err
}

func (p *ContractStateManager) GetContractState(address codec.Address) state.Mutable {
	return &prefixedStateMutable{prefix: accountStateKey(address), inner: p}
}

func (p *ContractStateManager) GetAccountContract(ctx context.Context, account codec.Address) (runtime.ContractID, error) {
	key, _ := keys.Encode(AccountContractKey(account), 36)
	result, err := p.GetValue(ctx, key)
	if err != nil {
		return ids.Empty[:], err
	}
	return result, nil
}

func (p *ContractStateManager) GetContractBytes(ctx context.Context, contractID runtime.ContractID) ([]byte, error) {
	return p.GetValue(ctx, contractID)
}

func (p *ContractStateManager) NewAccountWithContract(ctx context.Context, contractID runtime.ContractID, accountCreationData []byte) (codec.Address, error) {
	newAddress := GetAddressForDeploy(0, accountCreationData)
	key, _ := keys.Encode(AccountContractKey(newAddress), 36)
	_, err := p.GetValue(ctx, key)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return codec.EmptyAddress, err
	} else if err == nil {
		return codec.EmptyAddress, errors.New("account already exists")
	}

	return newAddress, p.SetAccountContract(ctx, newAddress, contractID)
}

func (p *ContractStateManager) SetAccountContract(ctx context.Context, account codec.Address, contractID runtime.ContractID) error {
	key, _ := keys.Encode(AccountContractKey(account), 36)
	return p.Insert(ctx, key, contractID)
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
