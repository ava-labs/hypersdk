// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/binary"
	"io"
	"math/big"
	"sort"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/shim"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/vm"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"go.uber.org/zap"
)

var _ chain.Action = (*EvmCall)(nil)

type EvmCall struct {
	To            *common.Address  `json:"to" rlp:"nil"` // nil means contract creation
	Nonce         uint64           `json:"nonce"`
	Value         *big.Int         `json:"value"`
	GasLimit      uint64           `json:"gasLimit"`
	GasPrice      *big.Int         `json:"gasPrice"`
	GasFeeCap     *big.Int         `json:"gasFeeCap"`
	GasTipCap     *big.Int         `json:"gasTipCap"`
	Data          []byte           `json:"data"`
	BlobGasFeeCap *big.Int         `json:"blobGasFeeCap"`
	BlobHashes    []common.Hash    `json:"blobHashes"`
	Keys          SerializableKeys `json:"stateKeys"`

	logger logging.Logger
}

func ToEVMAddress(addr codec.Address) common.Address {
	return common.BytesToAddress(addr[len(addr)-common.AddressLength:])
}

func (e *EvmCall) toMessage(from common.Address) *core.Message {
	// EVM state transition treats nil vs. empty blob hashes differently
	var blobHashes []common.Hash
	if len(e.BlobHashes) > 0 {
		blobHashes = e.BlobHashes
	}
	return &core.Message{
		From:          from,
		To:            e.To,
		Nonce:         e.Nonce,
		Value:         e.Value,
		GasLimit:      e.GasLimit,
		GasPrice:      e.GasPrice,
		GasFeeCap:     e.GasFeeCap,
		GasTipCap:     e.GasTipCap,
		Data:          e.Data,
		BlobGasFeeCap: e.BlobGasFeeCap,
		BlobHashes:    blobHashes,
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
	return state.Keys(e.Keys) // TODO: copy?
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
	_ ids.ID,
	_ bool,
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
		Time:        uint64(time),
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
	evmCall.Keys = make(SerializableKeys)
	if err := rlp.DecodeBytes(bytes, &evmCall); err != nil {
		return nil, err
	}
	return &evmCall, nil
}

func (*EvmCall) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (e *EvmCall) SetStateKeys(k state.Keys) {
	e.Keys = SerializableKeys(k)
}

func (e *EvmCall) SetLogger(logger logging.Logger) {
	e.logger = logger
}

type SerializableKeys state.Keys

type serializablePermissions struct {
	Key         string            `json:"key"`
	Permissions state.Permissions `json:"permission"`
}

func (s SerializableKeys) EncodeRLP(w io.Writer) error {
	keys := make([]serializablePermissions, 0, len(s))
	for k, v := range s {
		keys = append(keys, serializablePermissions{
			Key:         k,
			Permissions: v,
		})
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Key < keys[j].Key
	})
	return rlp.Encode(w, keys)
}

func (s SerializableKeys) DecodeRLP(stream *rlp.Stream) error {
	var keys []serializablePermissions
	if err := stream.Decode(&keys); err != nil {
		return err
	}
	for _, key := range keys {
		s[key.Key] = key.Permissions
	}
	return nil
}
