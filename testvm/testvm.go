// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/tstate"
)

type BlockProductionType int

const (
	Trigger BlockProductionType = iota
	PerTransactionBatch
	BlockTime
)

type BlockProduction struct {
	Type  BlockProductionType
	Value int
}

var (
	UnknownSnapshot       = errors.New("this snapshot doesn't exist")
	NotEnoughTransactions = errors.New("not enough pending transactions to fill batch")
	TooSoon               = errors.New("too soon to build a block")
)

var genesisBlock = chain.StatelessBlock{
	StatefulBlock: &chain.StatefulBlock{
		Prnt:   ids.ID{'n', 'o', 'n', 'e'},
		Tmstmp: 0,
		Hght:   0,
	},
}

var defaultEnv = Env{
	currentBlock: genesisBlock,
}

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

	pendingTransactions []chain.Transaction
	blockProduction     BlockProduction

	snapshots map[uint64]Snapshot
	rules     chain.MockRules
	maxUnits  fees.Dimensions
}

type TestConfig struct {
	*Env

	blockProduction BlockProduction
}

func (vm *TestVM) Init(config TestConfig, maxUnits fees.Dimensions) {
	if config.Env != nil {
		vm.Env = *config.Env
	} else {
		vm.Env = defaultEnv
	}

	vm.blockProduction = config.blockProduction
	vm.maxUnits = maxUnits
}

func (vm *TestVM) RunTransaction(ctx context.Context, tx chain.Transaction) (*chain.Result, error) {
	ts := tstate.New(1)
	sm := vm.StateManager()
	stateKeys, err := tx.StateKeys(sm)
	if err != nil {
		return nil, err
	}

	var storage = make(map[string][]byte, len(stateKeys))
	parent := vm.currentBlock
	parentView, err := parent.View(ctx, true)

	for k := range stateKeys {
		v, err := parentView.GetValue(ctx, []byte(k))
		if err != nil {
			return nil, err
		}
		storage[k] = v
	}

	tsv := ts.NewView(stateKeys, storage)
	nextTime := time.Now().UnixMilli()
	r := vm.Rules(nextTime)
	feeKey := chain.FeeKey(vm.StateManager().FeeKey())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return nil, err
	}

	parentFeeManager := fees.NewManager(feeRaw)
	feeManager, err := parentFeeManager.ComputeNext(nextTime, r)

	result, err := tx.Execute(
		ctx,
		feeManager,
		sm,
		r,
		tsv,
		nextTime,
	)
	if err != nil {
		return nil, err
	}

	if ok, _ := feeManager.Consume(result.Units, vm.maxUnits); !ok {
		return nil, nil
	}

	return result, nil
}

func (vm *TestVM) BuildBlock() error {
	switch vm.blockProduction.Type {
	case PerTransactionBatch:
		batch := vm.blockProduction.Value
		if batch > len(vm.pendingTransactions) {
			return NotEnoughTransactions
		}
	case BlockTime:
		blockTime := vm.blockProduction.Value
		currentTime := time.Now().Unix()
		if currentTime < vm.Timestamp()+int64(blockTime) {
			return TooSoon
		}
	}

	vm.Env.currentBlock = chain.StatelessBlock{
		StatefulBlock: &chain.StatefulBlock{
			Prnt:   vm.currentBlock.ID(),
			Tmstmp: vm.currentBlock.Tmstmp,
			Hght:   vm.currentBlock.Hght,
		},
	}

	return nil
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

// Parser
func (vm *TestVM) Rules(int64) chain.Rules {
	return &vm.rules
}

// VM
func (vm *TestVM) StateManager() chain.StateManager {
	return nil
}
