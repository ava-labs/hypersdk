// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ava-labs/coreth/accounts/abi"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	brpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ethereum/go-ethereum/common"
)

const maxGas = uint64(25000000)

type parsedABI struct {
	ABI      abi.ABI
	bytecode []byte
}

type rawJSON string

func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = rawJSON(data)
	return nil
}

func (r rawJSON) AsString() string {
	return string(r[1 : len(r)-1])
}

func NewABI(compiledFn string) (*parsedABI, error) {
	f, err := os.Open(compiledFn)
	if err != nil {
		return nil, err
	}

	mapData := make(map[string]rawJSON)
	if err := json.NewDecoder(f).Decode(&mapData); err != nil {
		return nil, err
	}

	bytecodeHex := mapData["bytecode"].AsString()
	bytecodeHex = strings.TrimLeft(bytecodeHex, "0x")
	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		return nil, err
	}

	abi, err := abi.JSON(strings.NewReader(string(mapData["abi"])))
	if err != nil {
		return nil, err
	}
	return &parsedABI{ABI: abi, bytecode: bytecode}, nil
}

type evmTxBuilder struct {
	bcli  *brpc.JSONRPCClient
	actor codec.Address
}

func (abi *parsedABI) deploy(args ...interface{}) ([]byte, error) {
	packed, err := abi.ABI.Pack("", args...)
	if err != nil {
		return nil, err
	}

	var bytecode []byte
	bytecode = append(bytecode, abi.bytecode...)
	bytecode = append(bytecode, packed...)
	return bytecode, nil
}

func (abi *parsedABI) calldata(method string, args ...interface{}) ([]byte, error) {
	if method == "" {
		return abi.deploy(args...)
	}
	return abi.ABI.Pack(method, args...)
}

type Args struct {
	To        *common.Address
	Value     *big.Int
	Data      []byte
	Gas       *uint64
	FillNonce bool
}

func (e *evmTxBuilder) getNonce(ctx context.Context) (uint64, error) {
	evmAddr := actions.ToEVMAddress(e.actor)
	_, nonce, err := e.bcli.EvmAccount(ctx, evmAddr.Hex())
	return nonce, err
}

func (e *evmTxBuilder) evmCall(ctx context.Context, args *Args) (*actions.EvmCall, error) {
	call := &actions.EvmCall{
		To:       args.To,
		GasLimit: maxGas,
		Value:    new(big.Int),

		// Gas is priced at 0
		GasPrice:  new(big.Int),
		GasFeeCap: new(big.Int),
		GasTipCap: new(big.Int),
	}
	if args.FillNonce {
		nonce, err := e.getNonce(ctx)
		if err != nil {
			return nil, err
		}
		call.Nonce = nonce
	}
	if args.Value != nil {
		call.Value = args.Value
	}
	if args.Data != nil {
		call.Data = args.Data
	}
	if args.Gas != nil {
		call.GasLimit = *args.Gas
	}

	trace, err := e.bcli.TraceAction(ctx, brpc.TraceTxArgs{
		Action: *call,
		Actor:  e.actor,
	})
	if err != nil {
		return nil, err
	}
	if !trace.Success {
		return nil, fmt.Errorf("call failed: %s", trace.Error)
	}
	p := codec.NewReader(trace.StateKeys, len(trace.StateKeys))
	call.Keys, err = actions.UnmarshalKeys(p)
	if err != nil {
		return nil, err
	}
	return call, nil
}
