// It is distributed under a license compatible with the licensing terms of the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********
// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see
// <http://www.gnu.org/licenses/>.

package ethrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	eth_rpc "github.com/ethereum/go-ethereum/rpc"
)

const Namespace = "eth"

var ed25519HexKey = "323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7" //nolint:lll

type EthRPCServer struct {
	vm      api.VM
	indexer *Indexer
}

func NewService(vm api.VM, indexer *Indexer) (*EthRPCServer, string) {
	return NewEthRPCServer(vm, indexer), Namespace
}

func NewEthRPCServer(vm api.VM, indexer *Indexer) *EthRPCServer {
	return &EthRPCServer{
		vm:      vm,
		indexer: indexer,
	}
}

type PingReply struct {
	Success bool `json:"success"`
}

func (e *EthRPCServer) Ping(_ *http.Request, _ *struct{}, reply *PingReply) error {
	e.vm.Logger().Info("ping")
	reply.Success = true
	return nil
}

type GetCodeArgs struct {
	Address   common.Address            `json:"address"`
	BlockInfo eth_rpc.BlockNumberOrHash `json:"blockInfo"`
}

func (args *GetCodeArgs) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}

	if len(params) != 2 {
		return fmt.Errorf("expected 2 parameters, got %d", len(params))
	}

	// Unmarshal address
	if err := json.Unmarshal(params[0], &args.Address); err != nil {
		return fmt.Errorf("failed to unmarshal address: %w", err)
	}

	// Unmarshal block info
	if err := json.Unmarshal(params[1], &args.BlockInfo); err != nil {
		return fmt.Errorf("failed to unmarshal block info: %w", err)
	}

	return nil
}

func (e *EthRPCServer) GetCode(req *http.Request, args *GetCodeArgs, reply *hexutil.Bytes) error {
	e.vm.Logger().Info("eth_getCode", zap.Any("address", *args))

	im, err := e.vm.ImmutableState(req.Context())
	if err != nil {
		return err
	}

	code, err := storage.GetCode(req.Context(), im, args.Address)
	if err != nil {
		return err
	}

	*reply = code
	return nil
}

type EstimateGasArgs struct {
	Tx        RPCTransaction            `json:"tx"`
	BlockInfo eth_rpc.BlockNumberOrHash `json:"blockInfo"`
}

func (args *EstimateGasArgs) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}

	if len(params) == 0 {
		return fmt.Errorf("expected at least 1 parameter, got 0")
	}

	// Unmarshal tx
	if err := json.Unmarshal(params[0], &args.Tx); err != nil {
		return fmt.Errorf("failed to unmarshal tx: %w", err)
	}

	if len(params) > 1 {
		// Unmarshal block info
		if err := json.Unmarshal(params[1], &args.BlockInfo); err != nil {
			return fmt.Errorf("failed to unmarshal block info: %w", err)
		}
	}

	return nil
}

func (e *EthRPCServer) EstimateGas(_ *http.Request, _ *EstimateGasArgs, reply *hexutil.Uint64) error {
	e.vm.Logger().Info("eth_estimateGas")

	// Overestimating for now
	*reply = hexutil.Uint64(300_000)

	return nil
}

type FeeHistoryArgs struct {
	BlockCount        math.HexOrDecimal64 `json:"blockCount"`
	NewestBlock       eth_rpc.BlockNumber `json:"newestBlock"`
	RewardPercentiles []float64           `json:"rewardPercentiles"`
}

func (args *FeeHistoryArgs) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}

	if len(params) != 3 {
		return fmt.Errorf("expected 3 parameters, got %d", len(params))
	}

	// Unmarshal block count
	if err := json.Unmarshal(params[0], &args.BlockCount); err != nil {
		return fmt.Errorf("failed to unmarshal block count: %w", err)
	}

	// Unmarshal newest block
	if err := json.Unmarshal(params[1], &args.NewestBlock); err != nil {
		return fmt.Errorf("failed to unmarshal newest block: %w", err)
	}

	// Unmarshal reward percentiles
	if err := json.Unmarshal(params[2], &args.RewardPercentiles); err != nil {
		return fmt.Errorf("failed to unmarshal reward percentiles: %w", err)
	}

	return nil
}

type FeeHistoryReply struct {
	OldestBlock      *hexutil.Big     `json:"oldestBlock"`
	Reward           [][]*hexutil.Big `json:"reward,omitempty"`
	BaseFee          []*hexutil.Big   `json:"baseFeePerGas,omitempty"`
	GasUsedRatio     []float64        `json:"gasUsedRatio"`
	BlobBaseFee      []*hexutil.Big   `json:"baseFeePerBlobGas,omitempty"`
	BlobGasUsedRatio []float64        `json:"blobGasUsedRatio,omitempty"`
}

func (e *EthRPCServer) FeeHistory(*http.Request, *FeeHistoryArgs, *FeeHistoryReply) error {
	e.vm.Logger().Info("eth_feeHistory")
	return nil
}

func (e *EthRPCServer) GasPrice(_ *http.Request, _ *struct{}, reply *hexutil.Big) error {
	e.vm.Logger().Info("eth_gasPrice")

	*reply = (hexutil.Big)(*big.NewInt(1))
	return nil
}

func (e *EthRPCServer) ChainID(_ *http.Request, _ *struct{}, reply *hexutil.Big) error {
	e.vm.Logger().Info("eth_chainID")

	*reply = (hexutil.Big)(*consts.ChainConfig.ChainID)
	return nil
}

type SimulateActionResult struct {
	Output    codec.Bytes `json:"output"`
	StateKeys state.Keys  `json:"stateKeys"`
}

func (e *EthRPCServer) SimulateTransaction(
	req *http.Request,
	rpcTx *RPCTransaction,
	reply *SimulateActionResult,
) error {
	e.vm.Logger().Info("simulateTransaction")

	// Convert TX to action before doing expensive work
	action, err := convertRPCTxToAction(rpcTx)
	if err != nil {
		return err
	}

	output, stateKeys, err := e.simulateAction(req.Context(), action)
	if err != nil {
		return err
	}
	reply.Output = output
	reply.StateKeys = stateKeys

	return nil
}

type GetBalanceArgs struct {
	Address   common.Address            `json:"address"`
	BlockInfo eth_rpc.BlockNumberOrHash `json:"blockInfo"`
}

func (args *GetBalanceArgs) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}

	if len(params) != 2 {
		return fmt.Errorf("expected 2 parameters, got %d", len(params))
	}

	// Unmarshal address
	if err := json.Unmarshal(params[0], &args.Address); err != nil {
		return fmt.Errorf("failed to unmarshal address: %w", err)
	}

	// Unmarshal block info
	if err := json.Unmarshal(params[1], &args.BlockInfo); err != nil {
		return fmt.Errorf("failed to unmarshal block info: %w", err)
	}

	return nil
}

func (e *EthRPCServer) GetBalance(req *http.Request, args *GetBalanceArgs, reply *hexutil.Big) error {
	e.vm.Logger().Info("eth_getBalance", zap.Any("address", args.Address))

	im, err := e.vm.ImmutableState(req.Context())
	if err != nil {
		return err
	}

	bal, err := storage.GetBalance(req.Context(), im, args.Address)
	if err != nil {
		return err
	}

	*reply = (hexutil.Big)(*new(big.Int).SetUint64(bal))
	return nil
}

type GetTransactionCountArgs struct {
	Address   common.Address            `json:"address"`
	BlockInfo eth_rpc.BlockNumberOrHash `json:"blockInfo"`
}

func (args *GetTransactionCountArgs) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}

	if len(params) != 2 {
		return fmt.Errorf("expected 2 parameters, got %d", len(params))
	}

	// Unmarshal address
	if err := json.Unmarshal(params[0], &args.Address); err != nil {
		return fmt.Errorf("failed to unmarshal address: %w", err)
	}

	// Unmarshal block info
	if err := json.Unmarshal(params[1], &args.BlockInfo); err != nil {
		return fmt.Errorf("failed to unmarshal block info: %w", err)
	}

	return nil
}

func (e *EthRPCServer) GetTransactionCount(req *http.Request, args *GetTransactionCountArgs, reply *hexutil.Uint64) error {
	e.vm.Logger().Info("eth_getTransactionCount", zap.Any("address", args.Address))

	im, err := e.vm.ImmutableState(req.Context())
	if err != nil {
		return err
	}

	nonce, err := storage.GetNonce(req.Context(), im, args.Address)
	if err != nil {
		return err
	}

	*reply = hexutil.Uint64(nonce)
	return nil
}

func (e *EthRPCServer) BlockNumber(req *http.Request, _ *struct{}, reply *hexutil.Uint64) error {
	e.vm.Logger().Info("eth_blockNumber")

	latestBlock, err := e.vm.LastAcceptedBlock(req.Context())
	if err != nil {
		*reply = hexutil.Uint64(0)
		return nil
	}
	*reply = hexutil.Uint64(latestBlock.GetHeight())
	e.vm.Logger().Info("eth_blockNumber returning", zap.Any("blockNumber", latestBlock.GetHeight()))
	return nil
}

type GetBlockByNumberArgs struct {
	BlockNumber eth_rpc.BlockNumber `json:"blockNumber"`
	FullTx      bool                `json:"fullTx"`
}

func (args *GetBlockByNumberArgs) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}

	if len(params) != 2 {
		return fmt.Errorf("expected 2 parameters, got %d", len(params))
	}

	// Unmarshal block number
	if err := json.Unmarshal(params[0], &args.BlockNumber); err != nil {
		return fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	// Unmarshal full tx
	if err := json.Unmarshal(params[1], &args.FullTx); err != nil {
		return fmt.Errorf("failed to unmarshal full tx: %w", err)
	}

	return nil
}

func (e *EthRPCServer) GetBlockByNumber(req *http.Request, args *GetBlockByNumberArgs, reply *map[string]interface{}) error {
	e.vm.Logger().Info("eth_getBlockByNumber", zap.Int64("blockNumber", args.BlockNumber.Int64()), zap.Bool("fullTx", args.FullTx))

	var height uint64
	if args.BlockNumber.Int64() < 0 {
		// Get the latest accepted block
		block, err := e.vm.LastAcceptedBlock(req.Context())
		if err != nil {
			return fmt.Errorf("failed to get latest accepted block: %w", err)
		}
		height = block.GetHeight()
	} else {
		height = uint64(args.BlockNumber.Int64())
	}

	var (
		found bool
		blk   *chain.ExecutedBlock
	)
	for i := 0; i < 10; i++ {
		b, err := e.indexer.GetBlockByHeight(height)
		if err == nil {
			blk = b
			found = true
			break
		}
		time.Sleep(time.Second)
	}

	if !found {
		return fmt.Errorf("block not found")
	}

	blockInfo := make(map[string]interface{})
	blockInfo["number"] = (*hexutil.Big)(big.NewInt(int64(blk.Block.GetHeight())))

	blockID := blk.Block.GetID()
	blockInfo["hash"] = common.BytesToHash(blockID[:])
	parentID := blk.Block.GetParent()
	blockInfo["parentHash"] = common.BytesToHash(parentID[:])

	txs := make([]common.Hash, 0)
	for _, tx := range blk.Block.Txs {
		txID := tx.GetID()
		txs = append(txs, common.BytesToHash(txID[:]))
	}
	blockInfo["transactions"] = txs
	blockRoot := blk.Block.GetStateRoot()
	blockInfo["stateRoot"] = common.BytesToHash(blockRoot[:])

	// Mocked values
	blockInfo["nonce"] = types.EncodeNonce(0)
	blockInfo["size"] = hexutil.Uint64(blk.Block.Size())
	blockInfo["sha3Uncles"] = common.Hash{}
	blockInfo["mixHash"] = common.Hash{}
	blockInfo["baseFeePerGas"] = "0x38ac7131"
	blockInfo["gasUsed"] = "0x1388"
	blockInfo["gasLimit"] = hexutil.Uint64(10_000_000)
	blockInfo["timestamp"] = hexutil.Uint64(blk.Block.GetTimestamp())
	blockInfo["miner"] = common.Address{}
	blockInfo["difficulty"] = (*hexutil.Big)(big.NewInt(0))
	blockInfo["uncles"] = []common.Hash{}
	blockInfo["transactionsRoot"] = common.Hash{}
	blockInfo["receiptsRoot"] = common.Hash{}
	blockInfo["logsBloom"] = types.Bloom{}
	blockInfo["totalDifficulty"] = (*hexutil.Big)(big.NewInt(0))
	blockInfo["extraData"] = hexutil.Bytes{}
	blockInfo["blockGasCost"] = (*hexutil.Big)(big.NewInt(0))
	blockInfo["blobGasUsed"] = (*hexutil.Big)(big.NewInt(0))
	blockInfo["excessBlobGas"] = hexutil.Uint64(0)
	blockInfo["parentBeaconBlockRoot"] = common.Hash{}

	*reply = blockInfo

	return nil
}

func (e *EthRPCServer) GetBlockByHash(_ *http.Request, _ *[]byte, _ *[]byte) error {
	e.vm.Logger().Info("eth_getBlockByHash")
	return nil
}

func (e *EthRPCServer) GetBlockReciepts(_ *http.Request, _ *[]byte, _ *[]byte) error {
	e.vm.Logger().Info("eth_getBlockReceipts")
	return nil
}

func (e *EthRPCServer) Call(req *http.Request, args *RPCTransaction, reply *hexutil.Bytes) error {
	e.vm.Logger().Info("eth_call", zap.Any("args", args))

	action, err := convertRPCTxToAction(args)
	if err != nil {
		return err
	}

	output, _, err := e.simulateAction(req.Context(), action)
	if err != nil {
		return err
	}

	*reply = hexutil.Bytes(output)
	return nil
}

// We assume that address == 0x784ce85b107389004d6a0e0d6d7518eeae1292d9
// TODO: figure out state keys
// If client doesn't specify state keys, we can compute for them
// However, this is clearly a DOS vector so this would need to be fleshed out
func (e *EthRPCServer) SendTransaction(req *http.Request, rpcTx *RPCTransaction, reply *common.Hash) error {
	e.vm.Logger().Info("eth_sendTransaction", zap.Any("tx", rpcTx))

	action, err := convertRPCTxToAction(rpcTx)
	if err != nil {
		return err
	}

	_, stateKeys, err := e.simulateAction(req.Context(), action)
	if err != nil {
		return err
	}
	action.Keys = stateKeys

	base := chain.Base{
		ChainID:   e.vm.ChainID(),
		Timestamp: time.Now().Unix()*1000 + 60*1000,
		MaxFee:    1_000_000,
	}
	bytes, err := codec.LoadHex(ed25519HexKey, ed25519.PrivateKeyLen)
	if err != nil {
		return err
	}
	txData := chain.NewTxData(base, []chain.Action{action})
	tx, err := txData.Sign(auth.NewED25519Factory(ed25519.PrivateKey(bytes)))
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}

	errs := e.vm.Submit(req.Context(), []*chain.Transaction{tx})
	if len(errs) != 0 && errs[0] != nil {
		e.vm.Logger().Info("got the following from vm", zap.Any("errors", errs))
		return errs[0]
	}
	txID := tx.GetID()
	*reply = common.BytesToHash(txID[:])
	return nil
}

func (e *EthRPCServer) SendRawTransaction(req *http.Request, args *hexutil.Bytes, reply *common.Hash) error {
	e.vm.Logger().Info("eth_sendRawTransaction", zap.Any("args", args))

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(*args); err != nil {
		return fmt.Errorf("failed to unmarshal tx: %w", err)
	}

	block, err := e.vm.LastAcceptedBlock(req.Context())
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	action, err := convertTxToAction(tx, block.GetHeight(), now)
	if err != nil {
		return fmt.Errorf("failed to convert tx to action: %w", err)
	}

	_, stateKeys, err := e.simulateAction(req.Context(), action)
	if err != nil {
		return fmt.Errorf("failed to simulate action: %w", err)
	}
	action.Keys = stateKeys

	signedAction := &actions.EvmSignedCall{
		Data: *args,
		Keys: stateKeys,
	}

	base := chain.Base{
		ChainID:   e.vm.ChainID(),
		Timestamp: time.Now().Unix()*1000 + 60*1000,
		MaxFee:    1_000_000,
	}
	bytes, err := codec.LoadHex(ed25519HexKey, ed25519.PrivateKeyLen)
	if err != nil {
		return err
	}
	txData := chain.NewTxData(base, []chain.Action{signedAction})
	hyperTx, err := txData.Sign(auth.NewED25519Factory(ed25519.PrivateKey(bytes)))
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}
	errs := e.vm.Submit(req.Context(), []*chain.Transaction{hyperTx})
	if len(errs) != 0 && errs[0] != nil {
		e.vm.Logger().Info("got the following from vm", zap.Any("errors", errs))
		return errs[0]
	}
	txID := hyperTx.GetID()
	*reply = common.BytesToHash(txID[:])

	return nil
}

func (e *EthRPCServer) GetTransactionReceipt(_ *http.Request, args *common.Hash, reply *map[string]interface{}) error {
	e.vm.Logger().Info("eth_getTransactionReceipt", zap.Any("txID", args))

	txInfo := make(map[string]interface{})
	txID := ids.ID(args.Bytes())

	var (
		found   bool
		success bool
		units   fees.Dimensions
		outputs [][]byte
	)
	for int := 0; int < 10; int++ {
		found, _, success, units, _, outputs, _, _ = e.indexer.GetTransaction(txID)
		if !found {
			time.Sleep(time.Second)
		}
	}

	if !found {
		return fmt.Errorf("transaction not found")
	}

	if len(outputs) != 1 {
		return fmt.Errorf("expected 1 output, got %d", len(outputs))
	}

	result, err := actions.UnmarshalEvmCallResult(outputs[0])
	if err != nil {
		return fmt.Errorf("failed to unmarshal call result: %w", err)
	}
	castResult, ok := result.(*actions.EvmCallResult)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	// Get block info
	height, ok := e.indexer.GetTransactionHeight(txID)
	if !ok {
		return fmt.Errorf("transaction height not found")
	}

	blk, err := e.indexer.GetBlockByHeight(height)
	if err != nil {
		return fmt.Errorf("failed to get block by height: %w", err)
	}

	if success {
		txInfo["status"] = hexutil.Uint(1)
	} else {
		txInfo["status"] = hexutil.Uint(0)
	}

	// Note to self: we probably need to look into the TX receipt/receipt hash
	txInfo["type"] = "0x00"
	txInfo["blockNumber"] = hexutil.Uint64(blk.Block.GetHeight())
	txInfo["contractAddress"] = castResult.ContractAddress
	blkID := blk.Block.GetID()
	txInfo["blockHash"] = common.BytesToHash(blkID[:])
	txInfo["gasUsed"] = "0x5208"
	txInfo["effectiveGasPrice"] = "0x1"
	txInfo["cumulativeGasUsed"] = hexutil.Uint64(castResult.UsedGas)
	txInfo["from"] = "0x784ce85b107389004d6a0e0d6d7518eeae1292d9"
	txInfo["transactionHash"] = *args
	txInfo["transactionIndex"] = "0x0"
	txInfo["units"] = units
	txInfo["logs"] = []*types.Log{}
	txInfo["logsBloom"] = "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	txInfo["result"] = castResult
	*reply = txInfo

	return nil
}

func convertTxToAction(tx *types.Transaction, height uint64, timestamp int64) (*actions.EvmCall, error) {
	// Convert TX to action before doing expensive work
	to := tx.To()
	var (
		isNullAddress = true
		toAddr        common.Address
	)
	if to != nil {
		isNullAddress = false
		toAddr = *to
	}
	chainConfig := params.SubnetEVMDefaultChainConfig
	chainConfig.ChainID = big.NewInt(0).Add(params.DefaultChainID, big.NewInt(1))
	signer := types.MakeSigner(chainConfig, big.NewInt(int64(height)), uint64(timestamp))
	from, err := types.Sender(signer, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}
	action := &actions.EvmCall{
		To:            toAddr,
		IsNullAddress: isNullAddress,
		Value:         tx.Value().Uint64(),
		GasLimit:      tx.Gas(),
		Data:          tx.Data(),
		From:          from,
	}

	return action, nil
}

// covertToAction takes an RPC transaction and converts it to an EVM call
func convertRPCTxToAction(tx *RPCTransaction) (*actions.EvmCall, error) {
	value := tx.Value.ToInt()
	if value == nil {
		value = big.NewInt(0)
	}
	if !value.IsUint64() {
		return nil, fmt.Errorf("value %s is too large", value.String())
	}

	var (
		toAddress     common.Address
		isNullAddress = true
	)
	if tx.To != nil {
		toAddress = *tx.To
		isNullAddress = false
	}

	return &actions.EvmCall{
		To:            toAddress,
		IsNullAddress: isNullAddress,
		Value:         value.Uint64(),
		GasLimit:      uint64(tx.Gas),
		Data:          tx.Input,
		From:          tx.From,
	}, nil
}

func (e *EthRPCServer) simulateAction(ctx context.Context, action chain.Action) (codec.Bytes, state.Keys, error) {
	currentState, err := e.vm.ImmutableState(ctx)
	if err != nil {
		return nil, nil, err
	}

	ts := tstate.New(0)
	scope := state.SimulatedKeys{}
	tsv := ts.NewView(
		scope,
		currentState,
		0,
	)

	currentTime := time.Now().UnixMilli()
	ruleFactory := e.vm.GetRuleFactory()
	rules := ruleFactory.GetRules(currentTime)
	actionCtx := chain.NewActionContext(0, currentTime, ids.Empty)
	actionOutput, err := action.Execute(
		ctx,
		actionCtx,
		rules,
		tsv,
		codec.EmptyAddress,
		ids.Empty,
	)
	e.vm.Logger().Info("executed action", zap.Any("output", actionOutput), zap.Error(err))

	return actionOutput, scope.StateKeys(), nil
}
