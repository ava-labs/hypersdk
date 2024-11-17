// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	utils "github.com/ava-labs/hypersdk/examples/morpheusvm/utils"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/registry"
	tworkload "github.com/ava-labs/hypersdk/tests/workload"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TestsRegistry initialized during init to ensure tests are identical during ginkgo
// suite construction and test execution
// ref https://onsi.github.io/ginkgo/#mental-model-how-ginkgo-traverses-the-spec-hierarchy
var TestsRegistry = &registry.Registry{}

func populateKeys(require *require.Assertions, cli *jsonrpc.JSONRPCClient, request *actions.EvmCall, spenderAddr codec.Address) (populatedCall *actions.EvmCall) {
	simRes, err := cli.SimulateActions(context.Background(), chain.Actions{request}, spenderAddr)
	require.NoError(err)
	require.Len(simRes, 1)
	actionResult := simRes[0]
	evmCallOutputBytes := actionResult.Output
	reader := codec.NewReader(evmCallOutputBytes, len(evmCallOutputBytes))
	evmCallResultTyped, err := vm.OutputParser.Unmarshal(reader)
	require.NoError(err)
	evmCallResult, ok := evmCallResultTyped.(*actions.EvmCallResult)
	require.True(ok)
	evmCallStateKeys := actionResult.StateKeys
	fmt.Println("evmCallResult.Return", evmCallResult.Success, evmCallResult.ErrorCode.String())
	require.True(evmCallResult.Success)
	request.Keys = evmCallStateKeys
	return request
}

var _ = registry.Register(TestsRegistry, "EVM Calls", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)

	toAddress := codec.CreateAddress(uint8(8), ids.GenerateTestID())
	toAddressEVM := storage.ConvertAddress(toAddress)

	networkConfig := tn.Configuration().(*workload.NetworkConfiguration)
	spendingKey := networkConfig.Keys()[0]

	spendingKeyAuthFactory := auth.NewED25519Factory(spendingKey)
	spendingKeyAddr := auth.NewED25519Address(networkConfig.Keys()[0].PublicKey())

	cli := jsonrpc.NewJSONRPCClient(tn.URIs()[0])
	lcli := vm.NewJSONRPCClient(tn.URIs()[0])
	indexerClient := indexer.NewClient(tn.URIs()[0])

	sufficientGas := uint64(1000000)

	testContractArtifact, err := utils.NewABI("../../contracts/artifacts/contracts/Test.sol/TestContract.json")
	if err != nil {
		panic(err)
	}

	t.Log("EVM tests - EOA to EOA transfer")

	// constructing the transfer transaction (which is an evm call action)
	transferCall := &actions.EvmCall{
		To:       &toAddressEVM,
		Value:    1,
		GasLimit: sufficientGas,
	}

	// populating the state keys using the simulator
	transferCall = populateKeys(require, cli, transferCall, spendingKeyAddr)

	// generating the transaction
	tx, err := tn.GenerateTx(context.Background(), []chain.Action{transferCall}, spendingKeyAuthFactory)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	// confirming the transaction
	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	// checking the transaction was included in the chain and succeeded
	response, included, err := indexerClient.GetTx(timeoutCtx, tx.ID())
	require.NoError(err)
	require.True(included)
	require.True(response.Success)

	// checking if the recipient received the correct amount of tokens
	balanceTo, err := lcli.Balance(timeoutCtx, toAddress)
	require.NoError(err)
	require.Equal(balanceTo, uint64(1))

	t.Log("EVM tests - contract deployment")
	deployCall := &actions.EvmCall{
		To:       &common.Address{}, // nil address for contract creation
		Value:    0,
		GasLimit: sufficientGas,
		Data:     testContractArtifact.Bytecode,
	}

	deployCall = populateKeys(require, cli, deployCall, spendingKeyAddr)
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{deployCall}, spendingKeyAuthFactory)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	response, included, err = indexerClient.GetTx(timeoutCtx, tx.ID())
	require.NoError(err)
	require.True(included)
	require.True(response.Success)

	t.Log("EVM tests - contract call (setValue)")

	value := big.NewInt(42)
	setValueData := testContractArtifact.ABI.Methods["setValue"].ID
	setValueData = append(setValueData, common.LeftPadBytes(value.Bytes(), 32)...)

	contractAddr := crypto.CreateAddress(storage.ConvertAddress(spendingKeyAddr), 1) // todo: nonce?

	setValueCall := &actions.EvmCall{
		To:       &contractAddr,
		Value:    0,
		GasLimit: sufficientGas,
		Data:     setValueData,
	}
	setValueCall = populateKeys(require, cli, setValueCall, spendingKeyAddr)
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{setValueCall}, spendingKeyAuthFactory)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	// checking the transaction was included in the chain and succeeded
	response, included, err = indexerClient.GetTx(timeoutCtx, tx.ID())
	require.NoError(err)
	require.True(included)
	require.True(response.Success)

	t.Log("EVM tests - contract call (getValue)")

	getValueData := testContractArtifact.ABI.Methods["getValue"].ID
	getValueCall := &actions.EvmCall{
		To:       &contractAddr,
		Value:    0,
		GasLimit: sufficientGas,
		Data:     getValueData,
	}
	getValueCall = populateKeys(require, cli, getValueCall, spendingKeyAddr)
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{getValueCall}, spendingKeyAuthFactory)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	response, included, err = indexerClient.GetTx(timeoutCtx, tx.ID())
	require.NoError(err)
	require.True(included)
	require.True(response.Success)

	// checking the output of the getValue call
	output := response.Outputs[0]
	reader := codec.NewReader(output, len(output))
	evmCallResultTyped, err := vm.OutputParser.Unmarshal(reader)
	require.NoError(err)
	evmCallResult, ok := evmCallResultTyped.(*actions.EvmCallResult)
	require.True(ok)
	require.Equal(common.LeftPadBytes(value.Bytes(), 32), evmCallResult.Return) // should be padded to 32 bytes

	t.Log("EVM tests - uniswap v2")

	var (
		abis      = make(map[string]*utils.ParsedABI)
		addresses = make(map[string]common.Address)
	)

	// Loading the contract artifacts
	tokenXArtifact, err := utils.NewABI("../../contracts/artifacts/contracts/TokenX.sol/TokenX.json")
	require.NoError(err)

	tokenYArtifact, err := utils.NewABI("../../contracts/artifacts/contracts/TokenY.sol/TokenY.json")
	require.NoError(err)

	weth9Artifact, err := utils.NewABI("../../contracts/node_modules/@uniswap/v2-periphery/build/WETH9.json")
	require.NoError(err)

	factoryArtifact, err := utils.NewABI("../../contracts/node_modules/@uniswap/v2-core/build/UniswapV2Factory.json")
	require.NoError(err)

	routerArtifact, err := utils.NewABI("../../contracts/node_modules/@uniswap/v2-periphery/build/UniswapV2Router02.json")
	require.NoError(err)

	// Setting the parameters for the swap
	owner := storage.ConvertAddress(spendingKeyAddr)
	// approvedAmount := big.NewInt(1_000_000_000_000_000_000)
	// minLiquidity := big.NewInt(1)
	// maxLiquidty := big.NewInt(1_000_000)
	// deadline := big.NewInt(time.Now().Unix() + 1000)

	abis["TokenX"] = tokenXArtifact
	abis["TokenY"] = tokenYArtifact
	abis["WETH9"] = weth9Artifact
	abis["Factory"] = factoryArtifact
	abis["Router"] = routerArtifact

	nonce := uint64(2)
	for name, abi := range abis {
		var deployData []byte
		deployData = abi.Bytecode

		if name == "Factory" {
			constructorArgs, err := abi.ABI.Pack("", owner) // empty string for constructor
			require.NoError(err)
			deployData = append(deployData, constructorArgs...)
		} else if name == "Router" {
			factoryAddr := addresses["Factory"]
			wethAddr := addresses["WETH9"]
			constructorArgs, err := abi.ABI.Pack("", factoryAddr, wethAddr)
			require.NoError(err)
			deployData = append(deployData, constructorArgs...)
		}

		deployCall := &actions.EvmCall{
			To:       &common.Address{}, // nil address for contract creation
			Value:    0,
			GasLimit: sufficientGas,
			Data:     deployData,
		}
		fmt.Println("deployCall", name)
		deployCall = populateKeys(require, cli, deployCall, spendingKeyAddr)
		tx, err = tn.GenerateTx(context.Background(), []chain.Action{deployCall}, spendingKeyAuthFactory)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

		addresses[name] = crypto.CreateAddress(owner, nonce)
		nonce++
	}

	// steps := []step{
	// 	{
	// 		abi: tokenXArtifact,
	// 	},
	// 	{
	// 		abi: tokenYArtifact,
	// 	},
	// 	{
	// 		abi: weth9Artifact,
	// 	},
	// 	{
	// 		abi: factoryArtifact,
	// 		args: func() []interface{} {
	// 			return []interface{}{owner}
	// 		},
	// 	},
	// 	{
	// 		abi: routerArtifact,
	// 		args: func() []interface{} {
	// 			return []interface{}{deployed["Factory"], deployed["WETH9"]}
	// 		},
	// 	},
	// 	{
	// 		abi: factoryArtifact,
	// 		method: "createPair",
	// 		args: func() []interface{} {
	// 			return []interface{}{deployed["TokenX"], deployed["TokenY"]}
	// 		},
	// 	},
	// 	{
	// 		abi: tokenXArtifact,
	// 		method: "approve",
	// 		args: func() []interface{} {
	// 			return []interface{}{deployed["Router"], approvedAmount}
	// 		},
	// 	},
	// 	{
	// 		abi: tokenYArtifact,
	// 		method: "approve",
	// 		args: func() []interface{} {
	// 			return []interface{}{deployed["Router"], approvedAmount}
	// 		},
	// 	},
	// 	{
	// 		abi: routerArtifact,
	// 		method: "addLiquidity",
	// 		args: func() []interface{} {
	// 			return []interface{}{
	// 				deployed["TokenX"],
	// 				deployed["TokenY"],
	// 				maxLiquidty,
	// 				maxLiquidty,
	// 				minLiquidity,
	// 				minLiquidity,
	// 				owner,
	// 				deadline,
	// 			}
	// 		},
	// 	},
	// }
	// for _, step := range steps {
	// 	nonce := uint64(2)
	// 	isDeploy := step.method == ""
	// 	var args []interface{}
	// 	if step.args != nil {
	// 		args = step.args()
	// 	}
	// 	calldata, err := step.abi.ABI.Pack(step.method, args...)
	// 	require.NoError(err)

	// 	var to *common.Address
	// 	if step.to != nil {
	// 		to = step.to()
	// 	} else if !isDeploy {
	// 		addr := deployed[
	// 		to = &addr
	// 	}
	// 	action := &actions.EvmCall{
	// 		To:       to,
	// 		Value:    0,
	// 		GasLimit: sufficientGas,
	// 		Data:     calldata,
	// 	}
	// 	action = populateKeys(require, cli, action, spendingKeyAddr)
	// 	require.NoError(err)

	// 	if isDeploy {
	// 		deployed[step.abi] = crypto.CreateAddress(owner, nonce)
	// 		nonce++
	// 	}

	// 	tx, err = tn.GenerateTx(context.Background(), []chain.Action{action}, spendingKeyAuthFactory)
	// 	require.NoError(err)

	// 	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
	// }
	t.Log("EVM tests - uniswap v2 all contracts deployed")

})
