// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"errors"

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

var (
	UnknownSnapshot = errors.New("this snapshot doesn't exist")
)

type Env struct {
	currentBlock chain.StatelessBlock
}

func (e *Env) Height() uint64 {
	return e.currentBlock.Height()
}

func (e *Env) Timestamp() int64 {
	return e.currentBlock.Timestamp().Unix()
}

func (e *Env) SetHeight(height uint64) {
	e.currentBlock.Hght = height
}

func (e *Env) SetTime(timestamp int64) {
	e.currentBlock.Tmstmp = timestamp
}

type Snapshot struct {
	Env
}

type TestVM struct {
	Env

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
	var index uint64
	for i := range vm.snapshots {
		if _, ok := vm.snapshots[i]; ok {
			index = i
			break
		}
	}
	return index
}

func (vm *TestVM) SnapshotDelete(id uint64) error {
	_, ok := vm.snapshots[id]
	if !ok {
		return UnknownSnapshot
	}

	delete(vm.snapshots, id)

	return nil
}

func (vm *TestVM) SnapshotRevert(id uint64) error {
	snapshot, ok := vm.snapshots[id]
	if !ok {
		return UnknownSnapshot
	}

	vm.Env = snapshot.Env

	return nil
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
