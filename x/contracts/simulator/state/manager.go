// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
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
func accountBalanceKey(account []byte) []byte {
	return accountDataKey(account, balanceKeyBytes)
}

// setContract stores [contract] at [contractID]
func (p *ContractStateManager) SetContract(
	ctx context.Context,
	contractID ids.ID,
	contract []byte,
) error {
	return p.db.Insert(ctx, contractKey(contractID[:]), contract)
}
