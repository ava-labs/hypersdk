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
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ runtime.StateManager = &ProgramStateManager{}

var (
	ErrUnknownAccount = errors.New("unknown account")
	balanceKeyBytes   = []byte("balance")
	programKeyBytes   = []byte("program")
)

const (
	programPrefix = 0x0

	accountPrefix      = 0x1
	accountDataPrefix  = 0x0
	accountStatePrefix = 0x1

	addressStoragePrefix = 0x3
)

type ProgramStateManager struct {
	db state.Mutable
}

func NewProgramStateManager(db state.Mutable) *ProgramStateManager {
	return &ProgramStateManager{db}
}

// GetBalance gets the balance associated [account].
// Returns 0 if no balance was found or errors if another error is present
func (p *ProgramStateManager) GetBalance(ctx context.Context, address codec.Address) (uint64, error) {
	return p.getAccountBalance(ctx, address)
}

func (p *ProgramStateManager) SetBalance(ctx context.Context, address codec.Address, amount uint64) error {
	return p.setAccountBalance(ctx, address, amount)
}

func (p *ProgramStateManager) TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error {
	fromBalance, err := p.getAccountBalance(ctx, from)
	if err != nil {
		return err
	}
	if fromBalance < amount {
		return errors.New("insufficient balance")
	}
	toBalance, err := p.getAccountBalance(ctx, to)
	if err != nil {
		return err
	}

	err = p.setAccountBalance(ctx, to, toBalance+amount)
	if err != nil {
		return err
	}

	return p.setAccountBalance(ctx, from, fromBalance-amount)
}

func (p *ProgramStateManager) GetProgramState(account codec.Address) state.Mutable {
	return newAccountPrefixedMutable(account, p.db)
}

// GetAccountProgram grabs the associated id with [account]. The ID is the key mapping to the programbytes
// Errors if there is no found account or an error fetching
func (p *ProgramStateManager) GetAccountProgram(ctx context.Context, account codec.Address) ([]byte, error) {
	programID, exists, err := p.getAccountProgram(ctx, account)
	if err != nil {
		return ids.Empty[:], err
	}
	if !exists {
		return ids.Empty[:], ErrUnknownAccount
	}
	return programID[:], nil
}

func (p *ProgramStateManager) GetProgramBytes(ctx context.Context, programID []byte) ([]byte, error) {
	// TODO: take fee out of balance?
	programBytes, exists, err := p.getProgram(ctx, programID)
	if err != nil {
		return []byte{}, ErrUnknownAccount
	}
	if !exists {
		return []byte{}, ErrUnknownAccount
	}
	return programBytes, nil
}

func (p *ProgramStateManager) NewAccountWithProgram(ctx context.Context, programID []byte, accountCreationData []byte) (codec.Address, error) {
	newID := sha256.Sum256(append(programID, accountCreationData...))
	newAccount := codec.CreateAddress(0, newID)
	return newAccount, p.setAccountProgram(ctx, newAccount, programID)
}

func (p *ProgramStateManager) SetAccountProgram(ctx context.Context, account codec.Address, programID []byte) error {
	return p.setAccountProgram(ctx, account, programID)
}

func (p *ProgramStateManager) setAccountBalance(ctx context.Context, account codec.Address, amount uint64) error {
	return p.db.Insert(ctx, accountBalanceKey(account[:]), binary.BigEndian.AppendUint64(nil, amount))
}

func (p *ProgramStateManager) getAccountBalance(ctx context.Context, account codec.Address) (uint64, error) {
	v, err := p.db.GetValue(ctx, accountBalanceKey(account[:]))
	if errors.Is(err, database.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(v), nil
}

// Creates an account balance key
func accountBalanceKey(account []byte) []byte {
	return accountDataKey(account, balanceKeyBytes)
}

func accountProgramKey(account []byte) []byte {
	return accountDataKey(account, programKeyBytes)
}

// Creates a key an account balance key
func accountDataKey(account []byte, key []byte) (k []byte) {
	// accountPrefix + account + accountDataPrefix + key
	k = make([]byte, 2+len(account)+len(key))
	k[0] = accountPrefix
	copy(k[1:], account)
	k[1+len(account)] = accountDataPrefix
	copy(k[1+len(account)+1:], key)
	return
}

func programKey(key []byte) (k []byte) {
	k = make([]byte, 1+len(key))
	k[0] = programPrefix
	copy(k[1:], key)
	return
}

func (p *ProgramStateManager) getAccountProgram(ctx context.Context, account codec.Address) (ids.ID, bool, error) {
	v, err := p.db.GetValue(ctx, accountProgramKey(account[:]))
	if errors.Is(err, database.ErrNotFound) {
		return ids.Empty, false, nil
	}
	if err != nil {
		return ids.Empty, false, err
	}
	return ids.ID(v[:32]), true, nil
}

func (p *ProgramStateManager) setAccountProgram(
	ctx context.Context,
	account codec.Address,
	programID []byte,
) error {
	return p.db.Insert(ctx, accountDataKey(account[:], programKeyBytes), programID)
}

// [programID] -> [programBytes]
func (p *ProgramStateManager) getProgram(ctx context.Context, programID []byte) ([]byte, bool, error) {
	v, err := p.db.GetValue(ctx, programKey(programID))
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// setProgram stores [program] at [programID]
func (p *ProgramStateManager) SetProgram(
	ctx context.Context,
	programID ids.ID,
	program []byte,
) error {
	return p.db.Insert(ctx, programKey(programID[:]), program)
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
