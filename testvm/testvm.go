// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/workers"
)

var _ TestVM = (*vm.VM)(nil)

// block production
const (
	PerTransactionBatch = iota
	BlockTime
	Trigger
)

type Snapshot struct{}

type TestVM struct {
	snapshots map[uint64]Snapshot
}

func (vm *TestVM) Init() {
	//
}

func (vm *TestVM) RunTransaction(tx chain.Transaction) {
	//
}

func (vm *TestVM) SetTimestamp(timestamp uint64) {
	//
}

func (vm *TestVM) SnapshotSave() uint64 {
	return 0
}

func (vm *TestVM) SnapshotRevert(id uint64) {
	//
}

// VM methods
func (vm *TestVM) AuthVerifiers() workers.Workers {
	return nil
}

func (vm *TestVM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	return nil, false
}

func (vm *TestVM) GetVerifyAuth() bool {
	return false
}
