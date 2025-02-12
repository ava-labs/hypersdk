// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/vmerrs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/evm/shim"
	"github.com/ava-labs/hypersdk/examples/evm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"

	econsts "github.com/ava-labs/hypersdk/examples/evm/consts"
)

var _ chain.Action = (*EvmCall)(nil)

type EvmCall struct {
	To             common.Address `serialize:"true" json:"to"` // Address of the contract to call (nil means contract creation)
	NotNullAddress bool           `serialize:"true" json:"notNullAddress"`
	Value          uint64         `serialize:"true" json:"value"`     // Amount of native tokens to send
	GasLimit       uint64         `serialize:"true" json:"gasLimit"`  // Maximum gas units to consume
	Data           []byte         `serialize:"true" json:"data"`      // Input data for the transaction
	Keys           state.Keys     `serialize:"true" json:"stateKeys"` // State keys accessed by this call
	From           common.Address `serialize:"true" json:"from"`      // Address of the sender
}

func (e *EvmCall) ComputeUnits(_ chain.Rules) uint64 {
	return e.GasLimit
}

func (e *EvmCall) toMessage(from common.Address) *core.Message {
	coreMessage := &core.Message{
		From:              from,
		To:                &e.To,
		Value:             big.NewInt(int64(e.Value)),
		Data:              e.Data,
		GasLimit:          e.GasLimit,
		GasPrice:          big.NewInt(0),
		GasFeeCap:         big.NewInt(0),
		GasTipCap:         big.NewInt(0),
		SkipAccountChecks: true, // Disables EVM state transition pre-check (nonce, EOA/prohibited addresses, and tx allow list)
	}
	if e.To == (common.Address{}) && !e.NotNullAddress {
		coreMessage.To = nil
	}
	return coreMessage
}

func (*EvmCall) GetTypeID() uint8 {
	return econsts.EvmCallID
}

func (e *EvmCall) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return e.Keys
}

// An error should only be returned if a fatal error was encountered, otherwise [success] should
// be marked as false and fees will still be charged.
func (e *EvmCall) Execute(
	ctx context.Context,
	blockCtx chain.BlockContext,
	r chain.Rules,
	mu state.Mutable,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	blockGasLimit := r.GetMaxBlockUnits()[fees.Compute]
	ethBlockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		GasLimit:    blockGasLimit,
		BlockNumber: new(big.Int).SetUint64(blockCtx.Height()),
		Time:        uint64(blockCtx.Timestamp()),
		Difficulty:  big.NewInt(1),
		BaseFee:     big.NewInt(0),
	}

	statedb, shim := shim.NewStateDB(ctx, mu)
	var from common.Address
	if e.From != (common.Address{}) {
		from = e.From
	} else {
		from = storage.ToEVMAddress(actor)
	}
	msg := e.toMessage(from)
	txContext := core.NewEVMTxContext(msg)

	evm := vm.NewEVM(
		ethBlockCtx, txContext, statedb, econsts.ChainConfig, vm.Config{},
	)
	gp := new(core.GasPool).AddGas(e.GasLimit)
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		fmt.Println("apply message error", err)
		return nil, err
	}
	if err := shim.Error(); err != nil {
		fmt.Println("shim error", err)
		return nil, err
	}
	if err := statedb.Error(); err != nil {
		fmt.Println("statedb error", err)
		return nil, err
	}
	_ = statedb.IntermediateRoot(true)

	var resultErrCode ErrorCode
	switch result.Err {
	case nil:
		resultErrCode = NilError
	case vmerrs.ErrExecutionReverted:
		resultErrCode = ErrExecutionReverted
	default:
		resultErrCode = ErrExecutionFailed
	}

	var contractAddress common.Address
	if msg.To == nil || *msg.To == (common.Address{}) {
		nonce := statedb.GetNonce(from)
		contractAddress = crypto.CreateAddress(from, nonce-1)
	}
	res := &EvmCallResult{
		Success:         result.Err == nil,
		Return:          result.ReturnData,
		UsedGas:         result.UsedGas,
		ErrorCode:       resultErrCode,
		ContractAddress: contractAddress,
	}
	return res, nil
}

var _ codec.Typed = (*EvmCallResult)(nil)

type EvmCallResult struct {
	Success         bool           `serialize:"true" json:"success"`
	Return          []byte         `serialize:"true" json:"return"`
	UsedGas         uint64         `serialize:"true" json:"usedGas"`
	ErrorCode       ErrorCode      `serialize:"true" json:"errorCode"`
	ContractAddress common.Address `serialize:"true" json:"contractAddress"`
}

// The result.Err field returned by core.ApplyMessage contains an error type, but
// the actual value is not part of the EVM's state transition function. ie. if the
// error changes it should not change the state transition (block/state).
// We convert it to an error code representing the three differentiated error types:
// nil (success), revert (special case), and all other erros as a generic failure.
type ErrorCode byte

const (
	NilError ErrorCode = iota
	ErrExecutionReverted
	ErrExecutionFailed
)

func (e ErrorCode) String() string {
	switch {
	case e == NilError:
		return "nil"
	case e == ErrExecutionReverted:
		return "reverted"
	case e == ErrExecutionFailed:
		return "failed"
	default:
		return "unknown"
	}
}

func (e ErrorCode) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

func (e *ErrorCode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "nil":
		*e = NilError
	case "reverted":
		*e = ErrExecutionReverted
	case "failed":
		*e = ErrExecutionFailed
	default:
		return fmt.Errorf("failed to unmarshal error code: %s", text)
	}
	return nil
}

func (*EvmCallResult) GetTypeID() uint8 {
	return econsts.EvmCallID
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
