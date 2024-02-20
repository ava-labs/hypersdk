// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package shim

import (
	"context"
	"math/big"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/subnet-evm/core"
	evm_state "github.com/ava-labs/subnet-evm/core/state"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
)

type EVMFactory struct {
	context  *vm.BlockContext
	gasLimit uint64
}

func NewEVMFactory() *EVMFactory {
	return &EVMFactory{}
}

func getParentHeight(ctx context.Context, im state.Immutable) (uint64, error) {
	return 0, nil
	// TODO: Implement this
	// parentHeightBytes, err := im.GetValue(ctx, storage.HeightKey())
	// if errors.Is(err, database.ErrNotFound) {
	// 	return 0, nil
	// } else if err != nil {
	// 	return 0, err
	// }
	// return binary.BigEndian.Uint64(parentHeightBytes), nil
}

func (e *EVMFactory) SetBlockContext(
	ctx context.Context,
	rules chain.Rules,
	im state.Immutable,
	time int64,
) error {
	blockGasLimit := rules.GetMaxBlockUnits()[1]
	parentHeight, err := getParentHeight(ctx, im)
	if err != nil {
		return err
	}
	e.context = &vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		GasLimit:    blockGasLimit,
		BlockNumber: new(big.Int).SetUint64(parentHeight + 1),
		Time:        uint64(time),
		Difficulty:  big.NewInt(1),
		BaseFee:     big.NewInt(0),
	}
	return nil
}

func (e *EVMFactory) BlockContext() *vm.BlockContext {
	return e.context
}

func (e *EVMFactory) GetEVM(
	_ context.Context, msg *core.Message, state *evm_state.StateDB,
	_ *types.Header, vmConfig *vm.Config, blockCtx *vm.BlockContext,
) (*vm.EVM, func() error) {
	e.gasLimit = msg.GasLimit
	if vmConfig == nil {
		vmConfig = &vm.Config{}
	}
	txContext := core.NewEVMTxContext(msg)
	if blockCtx == nil {
		blockCtx = e.context
	}
	return core.NewEVM(
		*blockCtx, txContext, state, params.SubnetEVMDefaultChainConfig, *vmConfig,
	), state.Error
}

func (e *EVMFactory) RPCGasCap() uint64 {
	return e.gasLimit
}
