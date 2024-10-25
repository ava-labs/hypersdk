// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var _ runtime.StateManager = &ContractStateManager{}

var (
	balanceKeyBytes       = []byte("balance")
	contractManagerPrefix = []byte("contract")
)

const (
	BalanceManagerPrefix = 0x1
)

type ContractStateManager struct {
	db            state.Mutable
	contractState *runtime.ContractStateManager
}

func NewContractStateManager(db state.Mutable) *ContractStateManager {
	contractManager := runtime.NewContractStateManager(db, contractManagerPrefix)
	return &ContractStateManager{
		db:            db,
		contractState: contractManager,
	}
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

	return database.ParseUInt64(v)
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

// Creates an account balance key
func accountBalanceKey(account []byte) (k []byte) {
	k = make([]byte, 0, 1+len(account)+len(balanceKeyBytes))
	k = append(k, BalanceManagerPrefix)
	k = append(k, account...)
	k = append(k, balanceKeyBytes...)
	return
}

// expose contract manager methods
func (p *ContractStateManager) GetContractState(address codec.Address) state.Mutable {
	return p.contractState.GetContractState(address)
}

func (p *ContractStateManager) GetAccountContract(ctx context.Context, account codec.Address) (runtime.ContractID, error) {
	return p.contractState.GetAccountContract(ctx, account)
}

func (p *ContractStateManager) GetContractBytes(ctx context.Context, contractID runtime.ContractID) ([]byte, error) {
	return p.contractState.GetContractBytes(ctx, contractID)
}

func (p *ContractStateManager) NewAccountWithContract(ctx context.Context, contractID runtime.ContractID, accountCreationData []byte) (codec.Address, error) {
	return p.contractState.NewAccountWithContract(ctx, contractID, accountCreationData)
}

func (p *ContractStateManager) SetAccountContract(ctx context.Context, account codec.Address, contractID runtime.ContractID) error {
	return p.contractState.SetAccountContract(ctx, account, contractID)
}

func (p *ContractStateManager) SetContractBytes(ctx context.Context, contractID runtime.ContractID, contractBytes []byte) error {
	return p.contractState.SetContractBytes(ctx, contractID, contractBytes)
}
