// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/binary"
	"math/big"
	"slices"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/shim"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

var _ chain.Action = (*EvmCall)(nil)

type EvmCall struct {
	To            *common.Address `json:"to"` // nil means contract creation
	Nonce         uint64          `json:"nonce"`
	Value         *big.Int        `json:"value"`
	GasLimit      uint64          `json:"gasLimit"`
	GasPrice      *big.Int        `json:"gasPrice"`
	GasFeeCap     *big.Int        `json:"gasFeeCap"`
	GasTipCap     *big.Int        `json:"gasTipCap"`
	Data          []byte          `json:"data"`
	BlobGasFeeCap *big.Int        `json:"blobGasFeeCap"`
	BlobHashes    []common.Hash   `json:"blobHashes"`
	Keys          state.Keys      `json:"stateKeys"`
	SkipNonces    bool            `json:"skipNonces"`

	logger         logging.Logger
	logReplayBytes bool
	usedGas        uint64
	executionError error
}

func ToEVMAddress(addr codec.Address) common.Address {
	hashed := hashing.ComputeHash256(addr[:])
	return common.BytesToAddress(hashed[len(hashed)-common.AddressLength:])
}

func (e *EvmCall) toMessage(from common.Address) *core.Message {
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
		BlobGasFeeCap:     e.BlobGasFeeCap,
		BlobHashes:        blobHashes,
		SkipAccountChecks: e.SkipNonces,
	}
}

func (*EvmCall) GetTypeID() uint8 {
	return mconsts.EvmCallID
}

func (e *EvmCall) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	if e.logger != nil {
		for k := range e.Keys {
			e.logger.Info(
				"EVM call state key",
				zap.String("key", common.Bytes2Hex([]byte(k))),
			)
		}
	}
	return e.Keys // TODO: copy?
}

func (e *EvmCall) StateKeysMaxChunks() []uint16 {
	output := make([]uint16, 0, len(e.Keys))
	for k := range e.Keys {
		bytes := []byte(k)
		// TODO: is there a helper for this?
		// TODO: tracer can measure the actual number of chunks
		maxChunks := binary.BigEndian.Uint16(bytes[:len(bytes)-2])
		output = append(output, maxChunks)
	}
	return output
}

func (*EvmCall) OutputsWarpMessage() bool {
	return false
}

func getParentHeight(_ context.Context, _ state.Immutable) (uint64, error) {
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

// An error should only be returned if a fatal error was encountered, otherwise [success] should
// be marked as false and fees will still be charged.
func (e *EvmCall) Execute(
	ctx context.Context,
	r chain.Rules,
	mu state.Mutable,
	time int64,
	actor codec.Address,
	txID ids.ID,
	warpVerified bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	blockGasLimit := r.GetMaxBlockUnits()[1]
	parentHeight, err := getParentHeight(ctx, mu)
	if err != nil {
		return false, 0, nil, nil, err
	}
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
	evm := core.NewEVM(
		blockCtx, txContext, statedb, chainConfig, vm.Config{},
	)
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
		args := []zap.Field{
			zap.Bool("success", result.Err == nil),
			zap.Uint64("gasUsed", result.UsedGas),
			zap.Binary("returnData", result.ReturnData),
			zap.Stringer("hash", hash),
			zap.Stringer("txID", txID),
			zap.NamedError("executionError", result.Err),
			zap.NamedError("statedbError", statedb.Error()),
		}
		if e.logReplayBytes {
			replayBytes := e.replayBytes(mu, time, actor, txID, warpVerified)
			args = append(
				args,
				zap.String("replayBytes", common.Bytes2Hex(replayBytes)),
			)
		}
		e.logger.Info("EVM call executed", args...)
	}
	success := result.Err == nil && statedb.Error() == nil
	e.executionError = result.Err
	if result.Err == nil {
		e.executionError = statedb.Error()
	}
	e.usedGas = result.UsedGas
	return success, GasToComputeUnits(result.UsedGas), result.ReturnData, nil, nil
}

func (e *EvmCall) MaxComputeUnits(chain.Rules) uint64 {
	return GasToComputeUnits(e.GasLimit)
}

func GasToComputeUnits(gas uint64) uint64 {
	return gas / 1000
}

func (e *EvmCall) Size() int {
	// TODO: try to calculate size without packing
	p := codec.NewWriter(0, consts.MaxInt)
	e.Marshal(p)
	return p.Offset()
}

func (e *EvmCall) Marshal(p *codec.Packer) {
	packBig := func(b *big.Int) {
		if b == nil {
			p.PackBytes([]byte{})
		} else {
			p.PackBytes(b.Bytes())
		}
	}

	if e.To == nil {
		p.PackBool(false)
	} else {
		p.PackBool(true)
		p.PackFixedBytes(e.To[:])
	}
	p.PackUint64(e.Nonce)
	packBig(e.Value)
	p.PackUint64(e.GasLimit)
	packBig(e.GasPrice)
	packBig(e.GasFeeCap)
	packBig(e.GasTipCap)
	if e.Data == nil {
		p.PackBytes([]byte{})
	} else {
		p.PackBytes(e.Data)
	}
	packBig(e.BlobGasFeeCap)
	p.PackInt(len(e.BlobHashes))
	for _, hash := range e.BlobHashes {
		p.PackFixedBytes(hash[:])
	}
	p.PackBool(e.SkipNonces)
	MarshalKeys(e.Keys, p)
}

func UnmarshalEvmCall(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	unlimited := -1
	var e EvmCall
	hasAddr := p.UnpackBool()
	if hasAddr {
		buf := make([]byte, common.AddressLength)
		p.UnpackFixedBytes(len(buf), &buf)
		e.To = new(common.Address)
		copy(e.To[:], buf)
	}
	e.Nonce = p.UnpackUint64(false)
	unpackBig := func() *big.Int {
		var buf []byte
		p.UnpackBytes(unlimited, false, &buf)
		return new(big.Int).SetBytes(buf)
	}
	e.Value = unpackBig()
	e.GasLimit = p.UnpackUint64(false)
	e.GasPrice = unpackBig()
	e.GasFeeCap = unpackBig()
	e.GasTipCap = unpackBig()
	p.UnpackBytes(unlimited, false, &e.Data)
	e.BlobGasFeeCap = unpackBig()
	numHashes := p.UnpackInt(false)
	e.BlobHashes = make([]common.Hash, numHashes)
	for i := 0; i < numHashes; i++ {
		buf := make([]byte, common.HashLength)
		p.UnpackFixedBytes(len(buf), &buf)
		copy(e.BlobHashes[i][:], buf)
	}
	e.SkipNonces = p.UnpackBool()
	var err error
	e.Keys, err = UnmarshalKeys(p)
	if err != nil {
		return nil, err
	}
	return &e, p.Err()
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (e *EvmCall) SetStateKeys(k state.Keys) {
	e.Keys = k
}

func (e *EvmCall) SetLogger(logger logging.Logger) {
	e.logger = logger
}

func (e *EvmCall) SetLogReplayBytes(logReplayBytes bool) {
	e.logReplayBytes = logReplayBytes
}

func (e *EvmCall) replayBytes(
	mu state.Mutable, time int64, actor codec.Address, txID ids.ID,
	warpVerified bool,
) []byte {
	ts, ok := mu.(*tstate.TStateView)
	if !ok {
		return []byte{}
	}
	p := codec.NewWriter(0, consts.MaxInt)
	e.Marshal(p)
	ts.Marshal(p)
	p.PackInt64(time)
	p.PackBytes(actor[:])
	p.PackBytes(txID[:])
	p.PackBool(warpVerified)
	return p.Bytes()
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

func MarshalKeys(s state.Keys, p *codec.Packer) {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	p.PackInt(len(keys))
	for _, k := range keys {
		p.PackString(k)
		p.PackByte(byte(s[k]))
	}
}

func UnmarshalKeys(p *codec.Packer) (state.Keys, error) {
	numKeys := p.UnpackInt(false)
	keys := make(state.Keys, numKeys)
	for i := 0; i < numKeys; i++ {
		key := p.UnpackString(false)
		perm := state.Permissions(p.UnpackByte())
		keys[key] = perm
	}
	return keys, p.Err()
}
