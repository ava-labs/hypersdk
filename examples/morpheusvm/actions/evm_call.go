// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math/big"
	"slices"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
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
	To            *common.Address `json:"to"`            // Address of the contract to call (nil means contract creation)
	Nonce         uint64          `json:"nonce"`         // Nonce for the transaction
	Value         *big.Int        `json:"value"`         // Amount of native tokens to send
	GasLimit      uint64          `json:"gasLimit"`      // Maximum gas units to consume
	GasPrice      *big.Int        `json:"gasPrice"`      // Price per unit of gas
	GasFeeCap     *big.Int        `json:"gasFeeCap"`     // Maximum fee per gas unit
	GasTipCap     *big.Int        `json:"gasTipCap"`     // Maximum tip per gas unit
	Data          []byte          `json:"data"`          // Input data for the transaction
	BlobGasFeeCap *big.Int        `json:"blobGasFeeCap"` // Maximum fee for blob gas
	BlobHashes    []common.Hash   `json:"blobHashes"`    // Hashes of associated blobs
	Keys          state.Keys      `json:"stateKeys"`     // State keys accessed by this call

	usedGas        uint64 // Gas used by the execution
	executionError error  // Error encountered during execution
}

func (e *EvmCall) ComputeUnits(r chain.Rules) uint64 {
	return GasToComputeUnits(e.GasLimit)
}

func ToEVMAddress(addr codec.Address) common.Address {
	hashed := hashing.ComputeHash256(addr[:])
	return common.BytesToAddress(hashed[len(hashed)-common.AddressLength:])
}

func (e *EvmCall) toMessage(from common.Address) *core.Message {
	return &core.Message{
		From:          from,
		To:            e.To,
		Nonce:         e.Nonce,
		Value:         e.Value,
		GasLimit:      e.GasLimit,
		GasPrice:      e.GasPrice,
		GasFeeCap:     e.GasFeeCap,
		GasTipCap:     e.GasTipCap,
		Data:          slices.Clone(e.Data),
		BlobGasFeeCap: e.BlobGasFeeCap,
		BlobHashes:    e.getBlobHashes(),
	}
}

func (e *EvmCall) getBlobHashes() []common.Hash {
	if len(e.BlobHashes) == 0 {
		return nil
	}
	return slices.Clone(e.BlobHashes)
}

func (*EvmCall) GetTypeID() uint8 {
	return mconsts.EvmCallID
}

func (e *EvmCall) StateKeys(actor codec.Address, txID ids.ID) state.Keys {
	// Make a deep copy to prevent external modification
	keys := make(state.Keys, len(e.Keys))
	for k, v := range e.Keys {
		keys[k] = v
	}
	return keys
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
	success := e.handleExecutionResult(result, statedb.Error())
	return &EvmCallResult{
		Success: success,
		UsedGas: GasToComputeUnits(result.UsedGas),
		Return:  result.ReturnData,
		Err:     e.executionError,
	}, nil
}

var _ codec.Typed = (*EvmCallResult)(nil)

type EvmCallResult struct {
	Success bool   `json:"success"`
	UsedGas uint64 `json:"usedGas"`
	Return  []byte `json:"return"`
	Err     error  `json:"err"`
}

func (e *EvmCallResult) GetTypeID() uint8 {
	return mconsts.EvmCallID
}

func (e *EvmCall) MaxComputeUnits(chain.Rules) uint64 {
	return GasToComputeUnits(e.GasLimit)
}

func GasToComputeUnits(gas uint64) uint64 {
	return gas / 1000
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (e *EvmCall) SetStateKeys(k state.Keys) {
	e.Keys = k
}

func (e *EvmCall) ExecutionError() string {
	if e.executionError == nil {
		return ""
	}
	return e.executionError.Error()
}

func (e *EvmCall) UsedGas() uint64 {
	return e.usedGas
}

func (e *EvmCall) Marshal(p *codec.Packer) {
	e.marshalAddress(p)
	e.marshalBasicFields(p)
	e.marshalBlobData(p)
	MarshalKeys(e.Keys, p)
}

func UnmarshalEvmCall(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var e EvmCall
	if err := e.unmarshalAddress(p); err != nil {
		return nil, err
	}
	if err := e.unmarshalBasicFields(p); err != nil {
		return nil, err
	}
	if err := e.unmarshalBlobData(p); err != nil {
		return nil, err
	}

	var err error
	e.Keys, err = UnmarshalKeys(p)
	if err != nil {
		return nil, err
	}
	return &e, p.Err()
}

func MarshalKeys(s state.Keys, p *codec.Packer) {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	p.PackInt(uint32(len(keys)))
	for _, k := range keys {
		p.PackString(k)
		p.PackByte(byte(s[k]))
	}
}

func UnmarshalKeys(p *codec.Packer) (state.Keys, error) {
	numKeys := p.UnpackInt(false)
	keys := make(state.Keys, numKeys)
	for i := 0; i < int(numKeys); i++ {
		key := p.UnpackString(false)
		perm := state.Permissions(p.UnpackByte())
		keys[key] = perm
	}
	return keys, p.Err()
}

func (e *EvmCall) handleExecutionResult(result *core.ExecutionResult, statedbErr error) bool {
	if result.Err != nil {
		e.executionError = result.Err
		return false
	}
	if statedbErr != nil {
		e.executionError = statedbErr
		return false
	}
	e.usedGas = result.UsedGas
	return true
}

func (e *EvmCall) marshalAddress(p *codec.Packer) {
	if e.To == nil {
		p.PackBool(false)
		return
	}
	p.PackBool(true)
	p.PackFixedBytes(e.To[:])
}

func (e *EvmCall) marshalBasicFields(p *codec.Packer) {
	p.PackUint64(e.Nonce)
	packBig(p, e.Value)
	p.PackUint64(e.GasLimit)
	packBig(p, e.GasPrice)
	packBig(p, e.GasFeeCap)
	packBig(p, e.GasTipCap)
	if e.Data == nil {
		p.PackBytes([]byte{})
	} else {
		p.PackBytes(e.Data)
	}
}

func (e *EvmCall) marshalBlobData(p *codec.Packer) {
	packBig(p, e.BlobGasFeeCap)
	p.PackInt(uint32(len(e.BlobHashes)))
	for _, hash := range e.BlobHashes {
		p.PackFixedBytes(hash[:])
	}
}

func packBig(p *codec.Packer, b *big.Int) {
	if b == nil {
		p.PackBytes([]byte{})
	} else {
		p.PackBytes(b.Bytes())
	}
}

func (e *EvmCall) unmarshalAddress(p *codec.Packer) error {
	hasAddr := p.UnpackBool()
	if hasAddr {
		buf := make([]byte, common.AddressLength)
		p.UnpackFixedBytes(len(buf), &buf)
		e.To = new(common.Address)
		copy(e.To[:], buf)
	}
	return p.Err()
}

func (e *EvmCall) unmarshalBasicFields(p *codec.Packer) error {
	e.Nonce = p.UnpackUint64(false)
	e.Value = unpackBig(p)
	e.GasLimit = p.UnpackUint64(false)
	e.GasPrice = unpackBig(p)
	e.GasFeeCap = unpackBig(p)
	e.GasTipCap = unpackBig(p)
	p.UnpackBytes(-1, false, &e.Data)
	return p.Err()
}

func (e *EvmCall) unmarshalBlobData(p *codec.Packer) error {
	e.BlobGasFeeCap = unpackBig(p)
	numHashes := p.UnpackInt(false)
	e.BlobHashes = make([]common.Hash, numHashes)
	for i := 0; i < int(numHashes); i++ {
		buf := make([]byte, common.HashLength)
		p.UnpackFixedBytes(len(buf), &buf)
		copy(e.BlobHashes[i][:], buf)
	}
	return p.Err()
}

// Helper function to unpack big.Int values
func unpackBig(p *codec.Packer) *big.Int {
	var buf []byte
	p.UnpackBytes(-1, false, &buf)
	return new(big.Int).SetBytes(buf)
}
