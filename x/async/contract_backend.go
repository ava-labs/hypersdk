// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package async

import (
	"errors"
	"fmt"

	"github.com/ava-labs/hypersdk/codec"
)

var _ EpochState = (*ContractBackend)(nil)

var errInvalidEpoch = errors.New("invalid epoch request")

// ContractBackend implements the Backend interface and could be implemented by a Solidity smart contract
// or EVM precompile.
type ContractBackend struct {
	currentEpoch   uint64
	lockedBalances map[codec.Address]uint64 // locked balances as of the end of currentEpoch - 2

	lastEpochDeposits           map[codec.Address]uint64 // Deposit requests made in currentEpoch - 1
	lastEpochWithdrawalRequests map[codec.Address]uint64 // Withdrawal requests made in currentEpoch - 1

	pendingDeposits     map[codec.Address]uint64 // Deposit requests made in currentEpoch
	pendingWithdrawals  map[codec.Address]uint64 // Withdrawal requests made in currentEpoch
	eligibleWithdrawals map[codec.Address]uint64
}

func NewContractBackend(epoch uint64) *ContractBackend {
	return &ContractBackend{
		currentEpoch:                epoch,
		lockedBalances:              make(map[codec.Address]uint64),
		lastEpochDeposits:           make(map[codec.Address]uint64),
		lastEpochWithdrawalRequests: make(map[codec.Address]uint64),
		eligibleWithdrawals:         make(map[codec.Address]uint64),
	}
}

func (c *ContractBackend) GetBalance(epoch uint64, address codec.Address) (uint64, error) {
	switch epoch {
	case c.currentEpoch:
		// If we're in the current epoch, return the locked balance at the close of currentEpoch - 2
		return c.lockedBalances[address], nil
	case c.currentEpoch + 1:
		// If we're in the next epoch, return the locked balance at the close of currentEpoch - 1
		// Compute this by taking close of currentEpoch - 2 and applying the diff from currentEpoch - 1.
		lockedBalance := c.lockedBalances[address]
		pendingWithdrawal := c.lastEpochWithdrawalRequests[address]
		pendingDeposit := c.lastEpochDeposits[address]
		return lockedBalance + pendingDeposit - pendingWithdrawal, nil
	default:
		return 0, fmt.Errorf("%w: cannot fetch balance of %s in epoch %d with current epoch %d", errInvalidEpoch, address, epoch, c.currentEpoch)
	}
}

func (c *ContractBackend) IncrementEpoch() {
	// Move the current epoch forward
	c.currentEpoch++

	// Process deposits/withdrawals to handle diff from currentEpoch - 1
	for addr, deposit := range c.lastEpochDeposits {
		c.lockedBalances[addr] += deposit
	}
	for addr, withdrawal := range c.lastEpochWithdrawalRequests {
		c.lockedBalances[addr] -= withdrawal
		c.eligibleWithdrawals[addr] += withdrawal
	}

	c.lastEpochDeposits = c.pendingDeposits
	c.lastEpochWithdrawalRequests = c.pendingWithdrawals

	clear(c.pendingDeposits)
	clear(c.pendingWithdrawals)
}

// TODO: implement user level functions to support creating accounts with an initial locked balance
// adding to their locked balance to increase allowed issuance, requesting, and processing withdrawals.
func (c *ContractBackend) CreateAccount(address codec.Address, initialLockedBalance uint64) {
	c.pendingDeposits[address] += initialLockedBalance
}

func (c *ContractBackend) AddBalance(address codec.Address, amount uint64) {
	c.pendingDeposits[address] += amount
}

func (c *ContractBackend) RequestWithdrawal(address codec.Address, amount uint64) {
	c.pendingWithdrawals[address] += amount
}

func (c *ContractBackend) ProcessWithdrawal(address codec.Address) uint64 {
	withdrawal, ok := c.eligibleWithdrawals[address]
	if !ok {
		return 0
	}
	delete(c.eligibleWithdrawals, address)
	return withdrawal
}
