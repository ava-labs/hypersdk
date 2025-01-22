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
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
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
	fmt.Println("evmCallResult", evmCallResult.Success, evmCallResult.ErrorCode.String())
	require.True(evmCallResult.Success)
	request.Keys = evmCallStateKeys
	return request
}

var _ = registry.Register(TestsRegistry, "EVM Calls", func(t ginkgo.FullGinkgoTInterface, tn tworkload.TestNetwork) {
	require := require.New(t)

	toAddress := codec.CreateAddress(uint8(8), ids.GenerateTestID())
	toAddressEVM := storage.ConvertAddress(toAddress)

	networkConfig := tn.Configuration()
	spendingKey := networkConfig.AuthFactories()[0]

	spendingKeyAddr := spendingKey.Address()

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
	tx, err := tn.GenerateTx(context.Background(), []chain.Action{transferCall}, spendingKey)
	require.NoError(err)

	timeoutCtx, timeoutCtxFnc := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer timeoutCtxFnc()

	// confirming the transaction
	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	// checking the transaction was included in the chain and succeeded
	response, included, err := indexerClient.GetTx(timeoutCtx, tx.GetID())
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
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{deployCall}, spendingKey)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	response, included, err = indexerClient.GetTx(timeoutCtx, tx.GetID())
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
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{setValueCall}, spendingKey)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	// checking the transaction was included in the chain and succeeded
	response, included, err = indexerClient.GetTx(timeoutCtx, tx.GetID())
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
	tx, err = tn.GenerateTx(context.Background(), []chain.Action{getValueCall}, spendingKey)
	require.NoError(err)

	require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

	response, included, err = indexerClient.GetTx(timeoutCtx, tx.GetID())
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

	evmTxBuilder := &utils.EvmTxBuilder{
		Actor: spendingKeyAddr,
		Lcli:  lcli,
		Cli:   cli,
	}
	t.Log("EVM tests - uniswap v2")
	var (
		abis     = make(map[string]*utils.ParsedABI)
		deployed = make(map[string]common.Address)
	)

	for name, fn := range map[string]string{
		"TokenA":  "../../contracts/artifacts/contracts/TokenX.sol/TokenX.json",
		"TokenB":  "../../contracts/artifacts/contracts/TokenY.sol/TokenY.json",
		"WETH9":   "../../contracts/node_modules/@uniswap/v2-periphery/build/WETH9.json",
		"Factory": "../../contracts/node_modules/@uniswap/v2-core/build/UniswapV2Factory.json",
		"Router":  "../../contracts/node_modules/@uniswap/v2-periphery/build/UniswapV2Router02.json",
	} {
		abi, err := utils.NewABI(fn)
		require.NoError(err)
		abis[name] = abi
	}

	type step struct {
		abi    string
		args   func() []interface{}
		method string
		to     func() *common.Address
	}

	owner := storage.ConvertAddress(spendingKeyAddr)
	approvedAmount := big.NewInt(1_000_000_000_000_000_000)
	minLiquidity := big.NewInt(1)
	maxLiquidty := big.NewInt(1_000_000)
	deadline := big.NewInt(time.Now().Unix() + 1000)

	steps := []step{
		{
			abi: "TokenA",
		},
		{
			abi: "TokenB",
		},
		{
			abi: "WETH9",
		},
		{
			abi: "Factory",
			args: func() []interface{} {
				return []interface{}{owner}
			},
		},
		{
			abi: "Router",
			args: func() []interface{} {
				return []interface{}{deployed["Factory"], deployed["WETH9"]}
			},
		},
		{
			abi:    "Factory",
			method: "createPair",
			args: func() []interface{} {
				return []interface{}{deployed["TokenA"], deployed["TokenB"]}
			},
		},
		{
			abi:    "TokenA",
			method: "approve",
			args: func() []interface{} {
				return []interface{}{deployed["Router"], approvedAmount}
			},
		},
		{
			abi:    "TokenB",
			method: "approve",
			args: func() []interface{} {
				return []interface{}{deployed["Router"], approvedAmount}
			},
		},
		{
			abi:    "Router",
			method: "addLiquidity",
			args: func() []interface{} {
				return []interface{}{
					deployed["TokenA"],
					deployed["TokenB"],
					maxLiquidty,
					maxLiquidty,
					minLiquidity,
					minLiquidity,
					owner,
					deadline,
				}
			},
		},
	}

	for _, step := range steps {
		isDeploy := step.method == ""
		var args []interface{}
		if step.args != nil {
			args = step.args()
		}
		calldata, err := abis[step.abi].Calldata(step.method, args...)
		require.NoError(err)

		var to *common.Address
		if step.to != nil {
			to = step.to()
		} else if !isDeploy {
			addr := deployed[step.abi]
			to = &addr
		}
		action, err := evmTxBuilder.EvmCall(context.Background(), &utils.Args{
			To:   to,
			Data: calldata,
		})
		require.NoError(err)

		if isDeploy {
			nonce, err := lcli.Nonce(context.Background(), spendingKeyAddr)
			require.NoError(err)
			deployed[step.abi] = crypto.CreateAddress(owner, nonce)
		}

		tx, err := tn.GenerateTx(context.Background(), []chain.Action{action}, spendingKey)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(context.Background(), []*chain.Transaction{tx}))
	}

	t.Log("EVM tests - uniswap v2 transfer erc20 tokens")

	amount := big.NewInt(1_000)
	tokenA := deployed["TokenA"]
	tokenB := deployed["TokenB"]

	for _, acct := range networkConfig.AuthFactories() {
		evmAddr := storage.ConvertAddress(acct.Address())
		calldataA, err := abis["TokenA"].Calldata("transfer", evmAddr, amount)
		require.NoError(err)
		actionA, err := evmTxBuilder.EvmCall(context.Background(), &utils.Args{
			To:   &tokenA,
			Data: calldataA,
		})
		require.NoError(err)
		tx, err := tn.GenerateTx(context.Background(), []chain.Action{actionA}, spendingKey)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

		calldataB, err := abis["TokenB"].Calldata("transfer", evmAddr, amount)
		require.NoError(err)
		actionB, err := evmTxBuilder.EvmCall(context.Background(), &utils.Args{
			To:   &tokenB,
			Data: calldataB,
		})
		require.NoError(err)
		tx, err = tn.GenerateTx(context.Background(), []chain.Action{actionB}, spendingKey)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
	}

	t.Log("EVM tests - approve erc20 tokens")

	amount = big.NewInt(10_000_000)

	for _, acct := range networkConfig.AuthFactories() {
		evmTxBuilder := &utils.EvmTxBuilder{
			Actor: acct.Address(),
			Lcli:  lcli,
			Cli:   cli,
		}

		calldataApproveA, err := abis["TokenA"].Calldata("approve", deployed["Router"], amount)
		require.NoError(err)
		actionAllowanceA, err := evmTxBuilder.EvmCall(context.Background(), &utils.Args{
			To:   &tokenA,
			Data: calldataApproveA,
		})
		require.NoError(err)
		tx, err = tn.GenerateTx(context.Background(), []chain.Action{actionAllowanceA}, spendingKey)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

		calldataApproveB, err := abis["TokenB"].Calldata("approve", deployed["Router"], amount)
		require.NoError(err)
		actionAllowanceB, err := evmTxBuilder.EvmCall(context.Background(), &utils.Args{
			To:   &tokenB,
			Data: calldataApproveB,
		})
		require.NoError(err)
		tx, err = tn.GenerateTx(context.Background(), []chain.Action{actionAllowanceB}, spendingKey)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))
	}

	t.Log("EVM tests - uniswap v2 swap erc20 tokens")
	router := deployed["Router"]
	routeAB := []common.Address{deployed["TokenA"], deployed["TokenB"]}
	routeBA := []common.Address{deployed["TokenB"], deployed["TokenA"]}

	amountOut := func(evmTxBuilder *utils.EvmTxBuilder, amountIn *big.Int, route []common.Address, slippageNum, slippageDen int) *big.Int {
		calldata, err := abis["Router"].Calldata("getAmountsOut", amountIn, route)
		require.NoError(err)
		action, err := evmTxBuilder.EvmCall(context.Background(), &utils.Args{
			To:   &router,
			Data: calldata,
		})
		require.NoError(err)

		tx, err := tn.GenerateTx(context.Background(), []chain.Action{action}, spendingKey)
		require.NoError(err)

		require.NoError(tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx}))

		response, included, err := indexerClient.GetTx(timeoutCtx, tx.GetID())
		require.NoError(err)
		require.True(included)
		require.True(response.Success)

		reader := codec.NewReader(response.Outputs[0], len(response.Outputs[0]))
		evmCallResultTyped, err := vm.OutputParser.Unmarshal(reader)
		require.NoError(err)
		evmCallResult, ok := evmCallResultTyped.(*actions.EvmCallResult)
		require.True(ok)

		var amountsOut []*big.Int
		err = abis["Router"].Unpack("getAmountsOut", evmCallResult.Return, &amountsOut)
		require.NoError(err)
		expected := amountsOut[len(amountsOut)-1]
		expected = expected.Mul(expected, big.NewInt(int64(slippageNum)))
		expected = expected.Div(expected, big.NewInt(int64(slippageDen)))
		return expected
	}

	balanceOf := func(evmTxBuilder *utils.EvmTxBuilder, name string, token common.Address) *big.Int {
		owner := storage.ConvertAddress(evmTxBuilder.Actor)
		calldata, err := abis[name].Calldata("balanceOf", owner)
		require.NoError(err)
		_, output, err := evmTxBuilder.EvmTraceCall(context.Background(), &utils.Args{
			To:   &token,
			Data: calldata,
		})
		require.NoError(err)

		var balance *big.Int
		err = abis[name].Unpack("balanceOf", output.Return, &balance)
		require.NoError(err)
		return balance
	}

	txBuilders := make([]*utils.EvmTxBuilder, len(networkConfig.AuthFactories()))
	for i, acct := range networkConfig.AuthFactories() {
		txBuilders[i] = &utils.EvmTxBuilder{
			Actor: acct.Address(),
			Lcli:  lcli,
			Cli:   cli,
		}
	}

	totalSuccess, totalTxs := 0, 0
	for round := 0; round < 20; round++ {
		amountIn := big.NewInt(200)
		// this avoids duplicate txs
		amountIn = amountIn.Sub(amountIn, big.NewInt(int64(round)))

		var minAmountOut *big.Int
		txs := 0
		success := 0
		for i, acct := range networkConfig.AuthFactories() {
			evmAddr := storage.ConvertAddress(acct.Address())

			var route []common.Address
			if (round+i)%2 == 0 {
				route = routeAB
			} else {
				route = routeBA
			}
			amountOut := amountOut(txBuilders[i], amountIn, route, 50, 100)
			if minAmountOut == nil || amountOut.Cmp(minAmountOut) < 0 {
				minAmountOut = amountOut
			}
			deadline := big.NewInt(time.Now().Unix() + 1000)
			calldata, err := abis["Router"].Calldata(
				"swapExactTokensForTokens",
				amountIn,
				amountOut,
				route,
				evmAddr,
				deadline,
			)
			require.NoError(err)
			var action *actions.EvmCall
			for attempt := 0; attempt < 100; attempt++ {
				action, err = txBuilders[i].EvmCall(context.Background(), &utils.Args{
					To:   &router,
					Data: calldata,
				})
				require.NoError(err)
				tx, err := tn.GenerateTx(context.Background(), []chain.Action{action}, acct)
				require.NoError(err)

				err = tn.ConfirmTxs(timeoutCtx, []*chain.Transaction{tx})
				require.NoError(err)
				response, included, err := indexerClient.GetTx(timeoutCtx, tx.GetID())
				require.NoError(err)
				if included && response.Success {
					success++
				}

				txs++
				if err != nil {
					balanceOfA := balanceOf(txBuilders[i], "TokenA", deployed["TokenA"])
					balanceOfB := balanceOf(txBuilders[i], "TokenB", deployed["TokenB"])
					fmt.Printf(
						"failed to create swap tx\n"+
							"try: %d\n"+
							"round: %d\n"+
							"account: %d\n"+
							"amountIn: %s\n"+
							"amountOut: %s\n"+
							"balanceA: %s\n"+
							"balanceB: %s\n",
						attempt, round, i, amountIn, amountOut, balanceOfA, balanceOfB,
					)
					continue
				}

				break
			}

		}
		// check balances
		totalA := big.NewInt(0)
		totalB := big.NewInt(0)
		totalNative := uint64(0)
		for i := range networkConfig.AuthFactories() {
			native, err := lcli.Balance(context.Background(), networkConfig.AuthFactories()[i].Address())
			require.NoError(err)
			balanceA := balanceOf(txBuilders[i], "TokenA", deployed["TokenA"])
			balanceB := balanceOf(txBuilders[i], "TokenB", deployed["TokenB"])
			totalA = totalA.Add(totalA, balanceA)
			totalB = totalB.Add(totalB, balanceB)
			totalNative += native
		}
		ratio := float64(minAmountOut.Uint64()) / float64(amountIn.Uint64())
		fmt.Printf(
			"swap round totals\n"+
				"round: %d\n"+
				"success: %d\n"+
				"txs: %d\n"+
				"TokenA: %s\n"+
				"TokenB: %s\n"+
				"ratio: %f\n"+
				"native: %d\n",
			round, success, txs, totalA, totalB, ratio, totalNative,
		)
		totalSuccess += success
		totalTxs += txs
	}
	fmt.Printf(
		"test summary\n"+
			"totalSuccess: %d\n"+
			"totalTxs: %d\n",
		totalSuccess, totalTxs,
	)

})
