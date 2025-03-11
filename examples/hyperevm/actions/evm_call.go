// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/vmerrs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/shim"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"

	econsts "github.com/ava-labs/hypersdk/examples/hyperevm/consts"
)

var _ chain.Action = (*EvmCall)(nil)

type EvmCall struct {
	To            common.Address `serialize:"true" json:"to"` // Address of the contract to call (nil means contract creation)
	IsNullAddress bool           `serialize:"true" json:"isNullAddress"`
	Value         uint64         `serialize:"true" json:"value"`     // Amount of native tokens to send
	GasLimit      uint64         `serialize:"true" json:"gasLimit"`  // Maximum gas units to consume
	Data          []byte         `serialize:"true" json:"data"`      // Input data for the transaction
	Keys          state.Keys     `serialize:"true" json:"stateKeys"` // State keys accessed by this call
	From          common.Address `serialize:"true" json:"from"`      // Address of the sender
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
) ([]byte, error) {
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

	statedb, shim, err := shim.NewStateDB(ctx, mu)
	if err != nil {
		return nil, err
	}
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
	return res.Bytes(), nil
}

func (e *EvmCall) ComputeUnits(_ chain.Rules) uint64 {
	return 1
}

func (e *EvmCall) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0),
		MaxSize: 1024,
	}
	p.PackByte(econsts.EvmCallID)
	_ = codec.LinearCodec.MarshalInto(e, p)
	return p.Bytes
}

func (*EvmCall) GetTypeID() uint8 {
	return econsts.EvmCallID
}

func (e *EvmCall) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return e.Keys
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
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
	if e.To == (common.Address{}) && !e.IsNullAddress {
		coreMessage.To = nil
	}
	return coreMessage
}

func UnmarshalEvmCall(bytes []byte) (chain.Action, error) {
	t := &EvmCall{}
	if len(bytes) == 0 {
		return nil, ErrUnmarshalEmptyEvmCall
	}
	if bytes[0] != consts.EvmCallID {
		return nil, fmt.Errorf("unexpected transfer typeID: %d != %d", bytes[0], consts.EvmCallID)
	}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

var _ codec.Typed = (*EvmCallResult)(nil)

type EvmCallResult struct {
	Success         bool           `serialize:"true" json:"success"`
	Return          []byte         `serialize:"true" json:"return"`
	UsedGas         uint64         `serialize:"true" json:"usedGas"`
	ErrorCode       ErrorCode      `serialize:"true" json:"errorCode"`
	ContractAddress common.Address `serialize:"true" json:"contractAddress"`
}

func (*EvmCallResult) GetTypeID() uint8 {
	return econsts.EvmCallID
}

func (e *EvmCallResult) Bytes() []byte {
	// TODO: fine-tune these values
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0),
		MaxSize: 1024,
	}
	p.PackByte(econsts.EvmCallID)
	_ = codec.LinearCodec.MarshalInto(e, p)
	return p.Bytes
}

func UnmarshalEvmCallResult(b []byte) (codec.Typed, error) {
	t := &EvmCallResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: b[1:]}, // XXX: first byte is guaranteed to be the typeID by the type parser
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}
