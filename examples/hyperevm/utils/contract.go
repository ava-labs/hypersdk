package contract

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/hypersdk/chain"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type ParsedABI struct {
	ABI              abi.ABI
	Bytecode         []byte
	DeployedBytecode []byte
}

type rawJSON string

func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = rawJSON(data)
	return nil
}

func (r rawJSON) AsString() string {
	return string(r[1 : len(r)-1])
}
func NewABI(compiledFn string) (*ParsedABI, error) {
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

	var deployedBytecode []byte
	if _, ok := mapData["deployedBytecode"]; ok {
		deployedBytecodeHex := mapData["deployedBytecode"].AsString()
		deployedBytecodeHex = strings.TrimLeft(deployedBytecodeHex, "0x")
		deployedBytecode, err = hex.DecodeString(deployedBytecodeHex)
		if err != nil {
			return nil, err
		}
	} else {
		deployedBytecode = bytecode
	}

	abi, err := abi.JSON(strings.NewReader(string(mapData["abi"])))
	if err != nil {
		return nil, err
	}
	return &ParsedABI{ABI: abi, Bytecode: bytecode, DeployedBytecode: deployedBytecode}, nil
}

func (p *ParsedABI) BytecodeHex() string {
	return hex.EncodeToString(p.Bytecode)
}

func (p *ParsedABI) DeployedBytecodeHex() string {
	return hex.EncodeToString(p.DeployedBytecode)
}

type EvmTxBuilder struct {
	Lcli  *vm.JSONRPCClient
	Cli   *jsonrpc.JSONRPCClient
	Actor codec.Address
}

func (abi *ParsedABI) Deploy(args ...interface{}) ([]byte, error) {
	packed, err := abi.ABI.Pack("", args...)
	if err != nil {
		return nil, err
	}

	var bytecode []byte
	bytecode = append(bytecode, abi.Bytecode...)
	bytecode = append(bytecode, packed...)
	return bytecode, nil
}

func (abi *ParsedABI) Calldata(method string, args ...interface{}) ([]byte, error) {
	if method == "" {
		return abi.Deploy(args...)
	}
	return abi.ABI.Pack(method, args...)
}

func (abi *ParsedABI) Unpack(method string, data []byte, res interface{}) error {
	if method == "" {
		method = "constructor"
	}
	return abi.ABI.UnpackIntoInterface(res, method, data)
}

type Args struct {
	To        *common.Address
	Value     *uint64
	Data      []byte
	Gas       *uint64
	FillNonce bool
}

func (e *EvmTxBuilder) GetNonce(ctx context.Context) (uint64, error) {
	return e.Lcli.Nonce(ctx, e.Actor)
}

func (e *EvmTxBuilder) EvmCall(ctx context.Context, args *Args) (*actions.EvmCall, error) {
	call, _, err := e.EvmTraceCall(ctx, args)
	if err != nil {
		return nil, err
	}
	return call, nil
}

// func populateKeys(require *require.Assertions, cli *jsonrpc.JSONRPCClient, request *actions.EvmCall, spenderAddr codec.Address) (populatedCall *actions.EvmCall) {
// 	simRes, err := cli.SimulateActions(context.Background(), chain.Actions{request}, spenderAddr)
// 	require.NoError(err)
// 	require.Len(simRes, 1)
// 	actionResult := simRes[0]
// 	evmCallOutputBytes := actionResult.Output
// 	reader := codec.NewReader(evmCallOutputBytes, len(evmCallOutputBytes))
// 	evmCallResultTyped, err := vm.OutputParser.Unmarshal(reader)
// 	require.NoError(err)
// 	evmCallResult, ok := evmCallResultTyped.(*actions.EvmCallResult)
// 	require.True(ok)
// 	evmCallStateKeys := actionResult.StateKeys
// 	fmt.Println("evmCallResult", evmCallResult.Success, evmCallResult.ErrorCode.String(), evmCallResult.Return)
// 	require.True(evmCallResult.Success)
// 	request.Keys = evmCallStateKeys
// 	return request
// }

func (e *EvmTxBuilder) EvmTraceCall(ctx context.Context, args *Args) (*actions.EvmCall, *actions.EvmCallResult, error) {
	const maxGas = uint64(25000000)

	call := &actions.EvmCall{
		To:       args.To,
		GasLimit: maxGas,
		Value:    0,
	}

	if args.To == nil {
		call.To = &common.Address{}
	}
	if args.Value != nil {
		call.Value = *args.Value
	}
	if args.Data != nil {
		call.Data = args.Data
	}
	if args.Gas != nil {
		call.GasLimit = *args.Gas
	}

	simRes, err := e.Cli.SimulateActions(ctx, chain.Actions{call}, e.Actor)
	if err != nil {
		return nil, nil, fmt.Errorf("simulate actions: %w", err)
	}
	actionResult := simRes[0]
	evmCallOutputBytes := actionResult.Output
	reader := codec.NewReader(evmCallOutputBytes, len(evmCallOutputBytes))
	evmCallResultTyped, err := vm.OutputParser.Unmarshal(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal evm call result: %w", err)
	}
	evmCallResult, ok := evmCallResultTyped.(*actions.EvmCallResult)
	if !ok {
		return nil, nil, fmt.Errorf("invalid evm call result")
	}
	evmCallStateKeys := actionResult.StateKeys

	call.Keys = evmCallStateKeys
	return call, evmCallResult, nil
}
