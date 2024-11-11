// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/shim"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
)

var _ chain.Action = (*EvmCall)(nil)

type EvmCall struct {
	To       *common.Address `serialize:"true" json:"to"`        // Address of the contract to call (nil means contract creation)
	Value    *big.Int        `serialize:"true" json:"value"`     // Amount of native tokens to send
	GasLimit uint64          `serialize:"true" json:"gasLimit"`  // Maximum gas units to consume
	Data     []byte          `serialize:"true" json:"data"`      // Input data for the transaction
	Keys     state.Keys      `serialize:"true" json:"stateKeys"` // State keys accessed by this call
}

func (e *EvmCall) ComputeUnits(_ chain.Rules) uint64 {
	return e.GasLimit
}

func ToEVMAddress(addr codec.Address) common.Address {
	hashed := hashing.ComputeHash256(addr[:])
	return common.BytesToAddress(hashed[len(hashed)-common.AddressLength:])
}

func (e *EvmCall) toMessage(from common.Address) *core.Message {
	return &core.Message{
		From:              from,
		To:                e.To,
		Value:             e.Value,
		GasLimit:          e.GasLimit,
		Data:              e.Data,
		GasPrice:          big.NewInt(0),
		GasFeeCap:         big.NewInt(0),
		GasTipCap:         big.NewInt(0),
		SkipAccountChecks: true, // Disables EVM state transition pre-check (nonce, EOA/prohibited addresses, and tx allow list)
	}
}

func (*EvmCall) GetTypeID() uint8 {
	return mconsts.EvmCallID
}

func (e *EvmCall) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return e.Keys
}

// An error should only be returned if a fatal error was encountered, otherwise [success] should
// be marked as false and fees will still be charged.
func (e *EvmCall) Execute(
	ctx context.Context,
	r chain.Rules,
	mu state.Mutable,
	time int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	blockGasLimit := r.GetMaxBlockUnits()[fees.Compute]
	parentHeight := uint64(0)
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		GasLimit:    blockGasLimit,
		BlockNumber: new(big.Int).SetUint64(parentHeight + 1),
		Time:        uint64(time / 1000),
		Difficulty:  big.NewInt(1),
		BaseFee:     big.NewInt(0),
	}

	statedb := shim.NewStateDB(ctx, mu)
	from := ToEVMAddress(actor)
	msg := e.toMessage(from)
	txContext := core.NewEVMTxContext(msg)
	chainConfig := params.SubnetEVMDefaultChainConfig
	evm := vm.NewEVM(
		blockCtx, txContext, statedb, chainConfig, vm.Config{},
	)
	gp := new(core.GasPool).AddGas(e.GasLimit)
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	// hash := statedb.IntermediateRoot(true)

	// NOTE: we must explicitly check the error from statedb, since if the tx
	// accesses a key that is not allowed, the EVM will not return an error
	// from ApplyMessage, but the statedb will have an error instead.
	return &EvmCallResult{
		Success: result.Err == nil,
		Return:  result.ReturnData,
		UsedGas: result.UsedGas,
		Err:     result.Err,
	}, statedb.Error()
}

var _ codec.Typed = (*EvmCallResult)(nil)

type EvmCallResult struct {
	Success bool   `json:"success"`
	UsedGas uint64 `json:"usedGas"`
	Return  []byte `json:"return"`
	Err     error  `json:"err"`
}

func (*EvmCallResult) GetTypeID() uint8 {
	return mconsts.EvmCallID
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
