// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	avatrace "github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/hypersdk/workers"
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

type Env struct {
	currentBlock chain.StatelessBlock

	storage map[string][]byte
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

var _ chain.VM = (*TestVM)(nil)

type TestVM struct {
	Env

	pendingTransactions []chain.Transaction
	blockProduction     BlockProduction

	snapshots    map[uint64]Snapshot
	rules        *chain.MockRules
	maxUnits     fees.Dimensions
	stateManager chain.StateManager
	tracer       avatrace.Tracer
}

type TestConfig struct {
	*Env
	BlockProduction BlockProduction
	MaxUnits        fees.Dimensions
	StateManager    chain.StateManager
	TracerConfig    trace.Config
	Rules           *chain.MockRules
}

func (vm *TestVM) Init(ctx context.Context, config TestConfig) error {
	if config.Env != nil {
		vm.Env = *config.Env
	} else {
		var genesisBlock = chain.NewBlock(
			vm, &chain.StatelessBlock{
				StatefulBlock: &chain.StatefulBlock{
					Prnt:      [32]byte{},
					Tmstmp:    0,
					Hght:      0,
					Txs:       []*chain.Transaction{},
					StateRoot: [32]byte{},
				},
			},
			0,
		)
		genesisBlock.MarkAccepted(ctx)

		var defaultEnv = Env{
			currentBlock: *genesisBlock,
			storage:      make(map[string][]byte),
		}

		vm.Env = defaultEnv
	}

	if vm.storage == nil {
		vm.storage = make(map[string][]byte)
	}

	vm.blockProduction = config.BlockProduction
	vm.maxUnits = config.MaxUnits
	vm.stateManager = config.StateManager
	tracer, err := trace.New(&config.TracerConfig)
	if err != nil {
		return err
	}
	vm.tracer = tracer
	vm.rules = config.Rules

	return nil
}

func (vm *TestVM) RunTransaction(ctx context.Context, tx chain.Transaction) (*chain.Result, error) {
	ts := tstate.New(1)
	sm := vm.StateManager()
	stateKeys, err := tx.StateKeys(sm)
	if err != nil {
		return nil, err
	}

	var tstorage = make(map[string][]byte, len(stateKeys))

	for k := range stateKeys {
		v, err := vm.Get([]byte(k))
		if err != nil {
			return nil, err
		}
		tstorage[k] = v
	}

	tsv := ts.NewView(stateKeys, tstorage)
	nextTime := time.Now().UnixMilli()
	r := vm.Rules(nextTime)
	// feeKey := chain.FeeKey(vm.StateManager().FeeKey())
	// feeRaw, err := vm.currentBlock.GetValue(ctx, feeKey)
	// if err != nil {
	// 	return nil, err
	// }

	// parentFeeManager := fees.NewManager(feeRaw)
	// feeManager, err := parentFeeManager.ComputeNext(nextTime, r)

	result, err := tx.Execute(
		ctx,
		fees.NewManager([]byte{}),
		sm,
		r,
		tsv,
		nextTime,
	)
	if err != nil {
		return nil, err
	}

	// if ok, _ := feeManager.Consume(result.Units, vm.maxUnits); !ok {
	// 	return nil, nil
	// }

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

func (vm *TestVM) Insert(k []byte, v []byte) {
	vm.storage[string(k)] = v
}

func (vm *TestVM) Get(k []byte) ([]byte, error) {
	v, ok := vm.storage[string(k)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return v, nil
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

func (vm *TestVM) SetStorage(key string) {
	// vm.currentBlock
}

// VM

// Metrics
func (vm *TestVM) RecordRootCalculated(time.Duration)          {}
func (vm *TestVM) RecordWaitRoot(time.Duration)                {}
func (vm *TestVM) RecordWaitSignatures(time.Duration)          {}
func (vm *TestVM) RecordBlockVerify(time.Duration)             {}
func (vm *TestVM) RecordBlockAccept(time.Duration)             {}
func (vm *TestVM) RecordStateChanges(int)                      {}
func (vm *TestVM) RecordStateOperations(int)                   {}
func (vm *TestVM) RecordBuildCapped()                          {}
func (vm *TestVM) RecordEmptyBlockBuilt()                      {}
func (vm *TestVM) RecordClearedMempool()                       {}
func (vm *TestVM) GetExecutorBuildRecorder() executor.Metrics  { return nil }
func (vm *TestVM) GetExecutorVerifyRecorder() executor.Metrics { return nil }

// Monitoring
func (vm *TestVM) Tracer() avatrace.Tracer { return vm.tracer }
func (vm *TestVM) Logger() logging.Logger  { return nil }

// Parser
func (vm *TestVM) Rules(int64) chain.Rules                              { return vm.rules }
func (vm *TestVM) Registry() (chain.ActionRegistry, chain.AuthRegistry) { return nil, nil }

func (vm *TestVM) AuthVerifiers() workers.Workers { return nil }
func (vm *TestVM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	return nil, false
}
func (vm *TestVM) GetVerifyAuth() bool { return false }

func (vm *TestVM) IsBootstrapped() bool                     { return false }
func (vm *TestVM) LastAcceptedBlock() *chain.StatelessBlock { return nil }
func (vm *TestVM) GetStatelessBlock(context.Context, ids.ID) (*chain.StatelessBlock, error) {
	return nil, nil
}

func (vm *TestVM) GetVerifyContext(ctx context.Context, blockHeight uint64, parent ids.ID) (chain.VerifyContext, error) {
	return nil, nil
}

func (vm *TestVM) State() (merkledb.MerkleDB, error) { return nil, nil }
func (vm *TestVM) StateManager() chain.StateManager  { return vm.stateManager }
func (vm *TestVM) ValidatorState() validators.State  { return nil }

func (vm *TestVM) Mempool() chain.Mempool { return nil }
func (vm *TestVM) IsRepeat(context.Context, []*chain.Transaction, set.Bits, bool) set.Bits {
	return set.Bits{}
}
func (vm *TestVM) GetTargetBuildDuration() time.Duration { return 0 }
func (vm *TestVM) GetTransactionExecutionCores() int     { return 0 }
func (vm *TestVM) GetStateFetchConcurrency() int         { return 0 }

func (vm *TestVM) Verified(context.Context, *chain.StatelessBlock) {}
func (vm *TestVM) Rejected(context.Context, *chain.StatelessBlock) {}
func (vm *TestVM) Accepted(context.Context, *chain.StatelessBlock) {}
func (vm *TestVM) AcceptedSyncableBlock(context.Context, *chain.SyncableBlock) (block.StateSyncMode, error) {
	return 0, nil
}

func (vm *TestVM) UpdateSyncTarget(*chain.StatelessBlock) (bool, error) { return false, nil }
func (vm *TestVM) StateReady() bool                                     { return false }
