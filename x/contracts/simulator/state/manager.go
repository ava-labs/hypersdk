// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var _ runtime.StateManager = &ContractStateManager{}

var (
	ErrUnknownAccount = errors.New("unknown account")
	balanceKeyBytes   = []byte("balance")
	contractKeyBytes  = []byte("contract")
)

const (
	contractPrefix = 0x0

	accountPrefix      = 0x1
	accountDataPrefix  = 0x0
	accountStatePrefix = 0x1

	addressStoragePrefix = 0x3
)

type ContractStateManager struct {
	db state.Mutable
}

func NewContractStateManager(db state.Mutable) *ContractStateManager {
	return &ContractStateManager{db}
}

// GetBalance gets the balance associated [account].
// Returns 0 if no balance was found or errors if another error is present
func (p *ContractStateManager) GetBalance(ctx context.Context, address codec.Address) (uint64, error) {
	v, err := p.db.GetValue(ctx, accountBalanceKey(address[:]))
	if errors.Is(err, database.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if len(v) != consts.Uint64Len {
		return 0, state.ErrMalformedEncoding
	}
	return binary.BigEndian.Uint64(v), nil
}

func (p *ContractStateManager) SetBalance(ctx context.Context, address codec.Address, amount uint64) error {
	return p.db.Insert(ctx, accountBalanceKey(address[:]), binary.BigEndian.AppendUint64(nil, amount))
}

func (p *ContractStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
	fromBalance, err := p.GetBalance(ctx, from)
	if err != nil {
		return err
	}
	if fromBalance < amount {
		return errors.New("insufficient balance")
	}
	toBalance, err := p.GetBalance(ctx, to)
	if err != nil {
		return err
	}

	err = p.SetBalance(ctx, to, toBalance+amount)
	if err != nil {
		return err
	}

	return p.SetBalance(ctx, from, fromBalance-amount)
}

func (p *ContractStateManager) GetContractState(account codec.Address) state.Mutable {
	return newAccountPrefixedMutable(account, p.db)
}

// GetAccountContract grabs the associated id with [account]. The ID is the key mapping to the contractbytes
// Errors if there is no found account or an error fetching
func (p *ContractStateManager) GetAccountContract(ctx context.Context, account codec.Address) (runtime.ContractID, error) {
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
func (p *ContractStateManager) GetContractBytes(ctx context.Context, contractID runtime.ContractID) ([]byte, error) {
	// TODO: take fee out of balance?
	contractBytes, err := p.db.GetValue(ctx, contractKey(contractID))
	if err != nil {
		return []byte{}, ErrUnknownAccount
	}

	return contractBytes, nil
}

func (p *ContractStateManager) NewAccountWithContract(ctx context.Context, contractID runtime.ContractID, accountCreationData []byte) (codec.Address, error) {
	newID := sha256.Sum256(append(contractID, accountCreationData...))
	newAccount := codec.CreateAddress(0, newID)
	return newAccount, p.SetAccountContract(ctx, newAccount, contractID)
}

func (p *ContractStateManager) SetAccountContract(ctx context.Context, account codec.Address, contractID runtime.ContractID) error {
	return p.db.Insert(ctx, accountDataKey(account[:], contractKeyBytes), contractID)
}

// Creates an account balance key
func accountBalanceKey(account []byte) []byte {
	return accountDataKey(account, balanceKeyBytes)
}

func accountContractKey(account []byte) []byte {
	return accountDataKey(account, contractKeyBytes)
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

func contractKey(key []byte) (k []byte) {
	k = make([]byte, 0, 1+len(key))
	k = append(k, contractPrefix)
	k = append(k, key...)
	return
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

// setContract stores [contract] at [contractID]
func (p *ContractStateManager) SetContract(
	ctx context.Context,
	contractID ids.ID,
	contract []byte,
) error {
	return p.db.Insert(ctx, contractKey(contractID[:]), contract)
}

// gets the public key mapped to the given name.
func GetPublicKey(ctx context.Context, db state.Immutable, name string) (ed25519.PublicKey, bool, error) {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k = append(k, addressStoragePrefix)
	k = append(k, []byte(name)...)

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
	k = append(k, addressStoragePrefix)
	k = append(k, []byte(name)...)
	return db.Insert(ctx, k, privateKey[:])
}
