// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// evmCallCmd flags
var (
	evmCallTo        string
	evmCallNonce     uint64
	evmCallValue     uint64
	evmCallGas       uint64
	evmCallData      string
	evmCallDataFile  string
	evmCallAbi       string
	evmCallMethod    string
	evmCallArgs      []string
	evmCallTraceOnly bool
)

var evmCallCmd = &cobra.Command{
	Use: "evm-call",
	RunE: func(*cobra.Command, []string) error {
		var to *common.Address
		if evmCallTo != "" {
			addr := common.HexToAddress(evmCallTo)
			to = &addr
		}
		data, err := readFileOrHex(evmCallData, evmCallDataFile)
		if err != nil {
			return err
		}
		if evmCallAbi != "" {
			calldata, err := prepareData(evmCallAbi, evmCallMethod, evmCallArgs)
			if err != nil {
				return err
			}
			utils.Outf("{{yellow}}calldata:{{/}} %s\n", hex.EncodeToString(calldata))
			data = append(data, calldata...)
		}
		call := &actions.EvmCall{
			To:       to,
			Nonce:    evmCallNonce,
			Value:    new(big.Int).SetUint64(evmCallValue),
			GasLimit: evmCallGas,
			Data:     data,

			// Gas is priced at 0
			GasPrice:  new(big.Int),
			GasFeeCap: new(big.Int),
			GasTipCap: new(big.Int),
		}
		// need to get the state keys
		_, priv, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}
		evmAddr := actions.ToEVMAddress(priv.Address)
		utils.Outf("{{yellow}}evm from:{{/}} %s\n", evmAddr.String())

		ctx := context.Background()
		trace, err := bcli.TraceAction(ctx, rpc.TraceTxArgs{
			Action: *call,
			Actor:  priv.Address,
		})
		if err != nil {
			return err
		}
		if trace.Success {
			p := codec.NewReader(trace.StateKeys, len(trace.StateKeys))
			call.Keys, err = actions.UnmarshalKeys(p)
			if err != nil {
				return err
			}
		}
		printTrace(trace, call.Keys)
		if !trace.Success || evmCallTraceOnly {
			return nil
		}

		if call.To == nil {
			utils.Outf(
				"{{yellow}}contract addr:{{/}} %s\n",
				crypto.CreateAddress(actions.ToEVMAddress(priv.Address), call.Nonce),
			)
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, call, cli, bcli, ws, factory, true)
		return err
	},
}

func makeEvmCallCmd() *cobra.Command {
	// TODO: BlobHashes & BlobGasFeeCap not implemented
	evmCallCmd.Flags().StringVar(&evmCallTo, "to", "", "Address to call")
	evmCallCmd.Flags().Uint64Var(&evmCallNonce, "nonce", 0, "Nonce of the transaction")
	evmCallCmd.Flags().Uint64Var(&evmCallValue, "value", 0, "Value of the transaction")
	evmCallCmd.Flags().Uint64Var(&evmCallGas, "gas", 0, "Gas limit of the transaction")
	evmCallCmd.Flags().StringVar(&evmCallData, "data", "", "Data to send with the transaction")
	evmCallCmd.Flags().StringVar(&evmCallDataFile, "data-file", "", "File to read data from")
	evmCallCmd.Flags().StringVar(&evmCallAbi, "abi", "", "ABI file for the contract")
	evmCallCmd.Flags().StringVar(&evmCallMethod, "method", "", "Method to call on the contract")
	evmCallCmd.Flags().StringSliceVar(&evmCallArgs, "args", []string{}, "Arguments to pass to the method")
	evmCallCmd.Flags().BoolVar(&evmCallTraceOnly, "trace-only", false, "Only trace the transaction")
	return evmCallCmd
}

func printTrace(reply *rpc.TraceTxReply, keys state.Keys) {
	utils.Outf("{{yellow}}evm-call trace: {{/}}")

	if reply.Success {
		utils.Outf("{{green}}success{{/}}")
	} else {
		utils.Outf("{{red}}failure: %s{{/}}", reply.Error)
	}

	utils.Outf(
		" - CUs: %d, Timestamp: %v, State keys: %d\n",
		reply.CUs,
		reply.Timestamp,
		len(keys),
	)
	if len(reply.Output) > 0 {
		utils.Outf(" - Output: %s\n", hex.EncodeToString(reply.Output))
	}
}

func readFileOrHex(hexStr, file string) ([]byte, error) {
	if file != "" {
		bytes, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		hexStr = string(bytes)
	}
	hexStr = strings.TrimPrefix(hexStr, "0x")
	hexStr = strings.TrimSpace(hexStr)
	return hex.DecodeString(hexStr)
}

func prepareData(abiFile string, method string, args []string) ([]byte, error) {
	file, err := os.Open(abiFile)
	if err != nil {
		return nil, err
	}
	parsed, err := abi.JSON(file)
	if err != nil {
		return nil, err
	}
	var (
		argsABI   []interface{}
		methodABI abi.Method
	)
	if method == "" {
		methodABI = parsed.Constructor
	} else {
		var ok bool
		methodABI, ok = parsed.Methods[method]
		if !ok {
			return nil, fmt.Errorf("method %s not found in ABI", method)
		}
	}
	if len(args) != len(methodABI.Inputs) {
		return nil, fmt.Errorf(
			"expected %d arguments, got %d",
			len(methodABI.Inputs),
			len(args),
		)
	}
	for i, arg := range methodABI.Inputs {
		switch arg.Type.T {
		case abi.AddressTy:
			addr := common.HexToAddress(args[i])
			argsABI = append(argsABI, addr)
		case abi.UintTy:
			bigInt := new(big.Int)
			bigInt, ok := bigInt.SetString(args[i], 0)
			if !ok {
				return nil, fmt.Errorf("invalid uint %s", args[i])
			}
			argsABI = append(argsABI, bigInt)
		default:
			return nil, fmt.Errorf("unsupported type %s", arg.Type)
		}
	}
	return parsed.Pack(method, argsABI...)
}
