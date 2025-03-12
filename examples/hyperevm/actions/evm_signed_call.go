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
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/vmerrs"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/shim"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"

	econsts "github.com/ava-labs/hypersdk/examples/hyperevm/consts"
)

var _ chain.Action = (*EvmSignedCall)(nil)

type EvmSignedCall struct {
	Data hexutil.Bytes `serialize:"true" json:"data"`      // Raw signed TX
	Keys state.Keys    `serialize:"true" json:"stateKeys"` // State keys accessed by this call
}

func (e *EvmSignedCall) Execute(
	ctx context.Context,
	actionCtx chain.ActionContext,
	r chain.Rules,
	mu state.Mutable,
	_ codec.Address,
	_ ids.ID,
) ([]byte, error) {
	// Convert to TX
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(e.Data); err != nil {
		return nil, err
	}
	// Get signer
	signer := types.MakeSigner(econsts.ChainConfig, new(big.Int).SetUint64(actionCtx.GetHeight()), uint64(actionCtx.GetTimestamp()))
	// Construct message
	baseFee := big.NewInt(0)
	msg, err := core.TransactionToMessage(tx, signer, baseFee)
	if err != nil {
		return nil, err
	}
	// TODO: remove this
	msg.SkipAccountChecks = true

	blockGasLimit := r.GetMaxBlockUnits()[fees.Compute]
	ethBlockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		GasLimit:    blockGasLimit,
		BlockNumber: new(big.Int).SetUint64(actionCtx.GetHeight()),
		Time:        uint64(actionCtx.GetTimestamp()),
		Difficulty:  big.NewInt(1),
		BaseFee:     baseFee,
	}

	statedb, shim, err := shim.NewStateDB(ctx, mu)
	if err != nil {
		return nil, err
	}
	statedb.SetTxContext(common.Hash(actionCtx.GetTxID()), 0)
	txContext := core.NewEVMTxContext(msg)

	evm := vm.NewEVM(
		ethBlockCtx, txContext, statedb, econsts.ChainConfig, vm.Config{},
	)
	gp := new(core.GasPool).AddGas(msg.GasLimit)
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
		nonce := statedb.GetNonce(msg.From)
		contractAddress = crypto.CreateAddress(msg.From, nonce-1)
	}
	logs := statedb.GetLogs(
		common.Hash(actionCtx.GetTxID()),
		actionCtx.GetHeight(),
		common.Hash(ids.Empty),
	)
	convertedLogs := convertLogs(logs)
	// TODO: populate [To] field
	res := &EvmActionResult{
		Success:         result.Err == nil,
		Return:          result.ReturnData,
		UsedGas:         result.UsedGas,
		ErrorCode:       resultErrCode,
		ContractAddress: contractAddress,
		Logs:            convertedLogs,
		From:            msg.From,
	}
	return res.Bytes(), nil
}

func (*EvmSignedCall) ComputeUnits(chain.Rules) uint64 {
	// TODO: make dynamic
	return 1
}

func (e *EvmSignedCall) Bytes() []byte {
	// TODO: fine-tune this
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0),
		MaxSize: 1024,
	}
	p.PackByte(econsts.EvmSignedCallID)
	_ = codec.LinearCodec.MarshalInto(e, p)
	return p.Bytes
}

func (*EvmSignedCall) GetTypeID() uint8 {
	return econsts.EvmSignedCallID
}

func (e *EvmSignedCall) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return e.Keys
}

func (*EvmSignedCall) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}
