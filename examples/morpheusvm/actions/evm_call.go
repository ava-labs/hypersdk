// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/shim"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"go.uber.org/zap"
)

var _ chain.Action = (*EvmCall)(nil)

type EvmCall struct {
	To            *common.Address `json:"to" rlp:"nil"` // nil means contract creation
	Nonce         uint64          `json:"nonce"`
	Value         *big.Int        `json:"value"`
	GasLimit      uint64          `json:"gasLimit"`
	GasPrice      *big.Int        `json:"gasPrice"`
	GasFeeCap     *big.Int        `json:"gasFeeCap"`
	GasTipCap     *big.Int        `json:"gasTipCap"`
	Data          []byte          `json:"data"`
	BlobGasFeeCap *big.Int        `json:"blobGasFeeCap"`
	BlobHashes    []common.Hash   `json:"blobHashes"`
	AccessList    *shim.Tracer    `json:"accessList"`

	logger         logging.Logger
	traceStateKeys bool
}

func tracerToAccessList(tracer *shim.Tracer, excl map[common.Address]struct{}) types.AccessList {
	var output types.AccessList
	if tracer == nil {
		return output
	}
	for addr, keys := range tracer.Writes {
		var storageKeys []common.Hash
		for key := range keys {
			storageKeys = append(storageKeys, key)
		}
		for key := range tracer.Reads[addr] {
			if _, ok := keys[key]; ok {
				continue
			}
			storageKeys = append(storageKeys, key)
		}
		if len(storageKeys) == 0 {
			if _, ok := excl[addr]; ok {
				continue
			}
		}
		output = append(output, types.AccessTuple{
			Address:     addr,
			StorageKeys: storageKeys,
		})
	}
	for addr, keys := range tracer.Reads {
		if _, ok := tracer.Writes[addr]; ok {
			continue
		}
		var storageKeys []common.Hash
		for key := range keys {
			storageKeys = append(storageKeys, key)
		}
		if len(storageKeys) == 0 {
			if _, ok := excl[addr]; ok {
				continue
			}
		}
		output = append(output, types.AccessTuple{
			Address:     addr,
			StorageKeys: storageKeys,
		})
	}
	return output
}

func ToEVMAddress(addr codec.Address) common.Address {
	return common.BytesToAddress(addr[len(addr)-common.AddressLength:])
}

func (e *EvmCall) toMessage(from common.Address) *core.Message {
	coinbase := common.Address{} // XXX: pass coinbase properly
	var to common.Address
	if e.To != nil {
		to = *e.To
	} else {
		to = crypto.CreateAddress(from, e.Nonce)
	}
	excl := map[common.Address]struct{}{
		from: {}, to: {}, coinbase: {},
	}

	// EVM state transition treats nil vs. empty blob hashes differently
	var blobHashes []common.Hash
	if len(e.BlobHashes) > 0 {
		blobHashes = e.BlobHashes
	}
	return &core.Message{
		From:              from,
		To:                e.To,
		Nonce:             e.Nonce,
		Value:             e.Value,
		GasLimit:          e.GasLimit,
		GasPrice:          e.GasPrice,
		GasFeeCap:         e.GasFeeCap,
		GasTipCap:         e.GasTipCap,
		Data:              e.Data,
		AccessList:        tracerToAccessList(e.AccessList, excl),
		BlobGasFeeCap:     e.BlobGasFeeCap,
		BlobHashes:        blobHashes,
		SkipAccountChecks: e.traceStateKeys,
	}
}

func (*EvmCall) GetTypeID() uint8 {
	return mconsts.EvmCallID
}

func (e *EvmCall) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	output := make(state.Keys)
	if e.AccessList == nil {
		return output
	}
	for addr, keys := range e.AccessList.Writes {
		codecAddress := storage.BytesToAddress(addr[:])
		output.Add(string(storage.BalanceKey(codecAddress)), state.Write)

		output.Add(string(storage.AccountKey(addr)), state.Write)
		for key := range keys {
			output.Add(string(storage.StorageKey(addr, key[:])), state.Write)
		}
	}
	for addr, keys := range e.AccessList.Reads {
		codecAddress := storage.BytesToAddress(addr[:])
		output.Add(string(storage.BalanceKey(codecAddress)), state.Read)

		output.Add(string(storage.AccountKey(addr)), state.Read)
		for key := range keys {
			output.Add(string(storage.StorageKey(addr, key[:])), state.Read)
		}
	}
	for addr := range e.AccessList.CodeWrites {
		output.Add(string(storage.CodeKey(addr)), state.Write)
	}
	for addr := range e.AccessList.CodeReads {
		output.Add(string(storage.CodeKey(addr)), state.Read)
	}

	if e.logger != nil {
		for k := range output {
			e.logger.Info(
				"EVM call state key",
				zap.String("key", common.Bytes2Hex([]byte(k))),
			)
		}
	}
	return output
}

func (e *EvmCall) StateKeysMaxChunks() []uint16 {
	var output []uint16
	if e.AccessList == nil {
		return output
	}
	for _, keys := range e.AccessList.Writes {
		output = append(output, storage.AccountChunks)
		for range keys {
			output = append(output, storage.StorageChunks)
		}
	}
	for account, keys := range e.AccessList.Reads {
		// only add account chunks if it's not already in the write set
		if _, ok := e.AccessList.Writes[account]; !ok {
			output = append(output, storage.AccountChunks)
		}
		for key := range keys {
			if _, ok := e.AccessList.Writes[account][key]; !ok {
				output = append(output, storage.StorageChunks)
			}
		}
	}
	for range e.AccessList.CodeWrites {
		output = append(output, storage.CodeChunks)
	}
	for account := range e.AccessList.CodeReads {
		if _, ok := e.AccessList.CodeWrites[account]; !ok {
			output = append(output, storage.CodeChunks)
		}
	}
	return output
}

func (*EvmCall) OutputsWarpMessage() bool {
	return false
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
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	f := shim.NewEVMFactory()
	if err := f.SetBlockContext(ctx, r, mu, time); err != nil {
		if e.logger != nil {
			e.logger.Error("EVM set block context failed", zap.Error(err))
		}
		return false, 0, nil, nil, err
	}

	var tracer *shim.Tracer
	if e.traceStateKeys {
		if e.AccessList == nil {
			e.AccessList = shim.NewTracer()
		}
		tracer = e.AccessList
	}
	statedb := shim.NewStateDBWithTracer(ctx, mu, tracer)
	from := ToEVMAddress(actor)
	msg := e.toMessage(from)
	evm, _ := f.GetEVM(ctx, msg, statedb, nil, nil, nil)
	gp := new(core.GasPool).AddGas(e.GasLimit)
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		if e.logger != nil {
			e.logger.Error("EVM call failed", zap.Error(err))
		}
		return false, 0, nil, nil, err
	}
	hash := statedb.IntermediateRoot(true)
	if e.logger != nil {
		e.logger.Info("EVM call executed",
			zap.Bool("success", result.Err == nil),
			zap.Uint64("gasUsed", result.UsedGas),
			zap.Binary("returnData", result.ReturnData),
			zap.Stringer("hash", hash),
			zap.Error(result.Err),
		)
	}
	success := result.Err == nil
	return success, GasToComputeUnits(result.UsedGas), result.ReturnData, nil, nil
}

func (e *EvmCall) MaxComputeUnits(chain.Rules) uint64 {
	return GasToComputeUnits(e.GasLimit)
}

func GasToComputeUnits(gas uint64) uint64 {
	return gas / 1000
}

func (*EvmCall) Size() int {
	// TODO: try to calculate size without packing
	p := codec.NewWriter(0, consts.MaxInt)
	return p.Offset()
}

func (e *EvmCall) Marshal(p *codec.Packer) {
	// TODO: use packer
	bytes, err := rlp.EncodeToBytes(e)
	if err != nil {
		panic(err)
	}
	p.PackBytes(bytes)
}

func UnmarshalEvmCall(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	unlimited := -1
	var bytes []byte
	p.UnpackBytes(unlimited, false, &bytes)
	if p.Err() != nil {
		return nil, p.Err()
	}
	var evmCall EvmCall
	if err := rlp.DecodeBytes(bytes, &evmCall); err != nil {
		return nil, err
	}
	return &evmCall, nil
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (e *EvmCall) TraceAction() chain.Action {
	return &evmCallTracer{e}
}

type evmCallTracer struct {
	*EvmCall
}

func (e *evmCallTracer) Execute(
	ctx context.Context,
	r chain.Rules,
	mu state.Mutable,
	time int64,
	actor codec.Address,
	txID ids.ID,
	warpVerified bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	// enable tracing of state keys
	original := e.EvmCall.traceStateKeys
	e.EvmCall.traceStateKeys = true
	defer func() { e.EvmCall.traceStateKeys = original }()

	var (
		stateKeys   = e.StateKeys(actor, txID)
		success     bool
		actionCUs   uint64
		output      []byte
		warpMessage *warp.UnsignedMessage
		err         error
	)
	// calculate a fixpoint of state keys
	// since state keys listed impact the evm tx access list, they
	// can impact gas calculation and tx execution path.
	for {
		success, actionCUs, output, warpMessage, err = e.EvmCall.Execute(
			ctx, r, mu, time, actor, txID, warpVerified)
		if err != nil {
			return false, 0, nil, nil, err
		}
		nextStateKeys := e.StateKeys(actor, txID)
		if StateKeysEqual(nextStateKeys, stateKeys) {
			break
		}
		stateKeys = nextStateKeys
	}
	return success, actionCUs, output, warpMessage, nil
}

func (e *EvmCall) SetLogger(logger logging.Logger) {
	e.logger = logger
}

// TODO: move to state package
func StateKeysEqual(k state.Keys, other state.Keys) bool {
	if len(k) != len(other) {
		return false
	}
	for key, perm := range k {
		if other[key] != perm {
			return false
		}
	}
	return true
}
