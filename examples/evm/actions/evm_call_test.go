// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/examples/evm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestSerialization(t *testing.T) {
	require := require.New(t)

	sender := common.Address{1}
	t.Log("testing serialization - empty To")
	evmCall := &EvmCall{
		To:       common.Address{},
		Value:    1,
		GasLimit: 1000000,
		Data:     []byte{},
	}
	msg := evmCall.toMessage(sender)
	require.True(msg.To == nil)

	t.Log("testing serialization - non-empty To")
	evmCall = &EvmCall{
		To:             common.Address{1},
		NotNullAddress: true,
		Value:          1,
		GasLimit:       1000000,
		Data:           []byte{},
	}
	msg = evmCall.toMessage(sender)
	require.NotNil(msg.To)

	t.Log("testing serialization - value")
	evmCall = &EvmCall{
		To:             common.Address{1},
		NotNullAddress: true,
		Value:          10,
		GasLimit:       1000000,
		Data:           []byte{},
	}
	msg = evmCall.toMessage(sender)
	require.IsType(msg.Value, &big.Int{})
}

func TestDeployment(t *testing.T) {
	require := require.New(t)

	testCtx := NewTestContext()
	blockContext := chain.NewBlockContext(0, testCtx.Timestamp)

	firstDeployTest := &chaintest.ActionTest{
		Name: "deploy contract",
		Action: &EvmCall{
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     testCtx.TestContractABI.Bytecode,
			Keys:     state.Keys{},
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: blockContext,
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:         true,
			UsedGas:         0x8459a,
			Return:          testCtx.TestContractABI.DeployedBytecode,
			ErrorCode:       NilError,
			ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			code, err := storage.GetCode(ctx, mu, crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce))
			require.NoError(err)
			require.NotEmpty(code)
			require.ElementsMatch(code, testCtx.TestContractABI.DeployedBytecode)
		},
	}
	firstDeployTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
	testCtx.Nonce++

	secondDeployTest := &chaintest.ActionTest{
		Name: "deploy same contract again",
		Action: &EvmCall{
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     testCtx.TestContractABI.Bytecode,
		},
		BlockCtx: blockContext,
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:         true,
			UsedGas:         0x8459a,
			Return:          testCtx.TestContractABI.DeployedBytecode,
			ErrorCode:       NilError,
			ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			code, err := storage.GetCode(ctx, mu, crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce))
			require.NoError(err)
			require.NotEmpty(code)
			require.ElementsMatch(code, testCtx.TestContractABI.DeployedBytecode)
		},
	}
	secondDeployTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
	testCtx.Nonce++

	factoryDeployTest := &chaintest.ActionTest{
		Name: "deploy factory contract",
		Action: &EvmCall{
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     testCtx.FactoryABI.Bytecode,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: blockContext,
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:         true,
			UsedGas:         0x98bf6,
			Return:          testCtx.FactoryABI.DeployedBytecode,
			ErrorCode:       NilError,
			ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			code, err := storage.GetCode(ctx, mu, crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce))
			require.NoError(err)
			require.NotEmpty(code)
			require.ElementsMatch(code, testCtx.FactoryABI.DeployedBytecode)
		},
	}
	factoryDeployTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
	testCtx.Nonce++

	factoryAddr := crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce) // we just deployed the factory
	deployData := testCtx.FactoryABI.ABI.Methods["deployContract"].ID
	deployFromFactoryTest := &chaintest.ActionTest{
		Name: "deploy contract from a contract",
		Action: &EvmCall{
			To:             factoryAddr,
			NotNullAddress: true,
			Value:          0,
			GasLimit:       testCtx.SufficientGas,
			Data:           deployData,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: blockContext,
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:         true,
			UsedGas:         0x5248,
			Return:          nil,
			ErrorCode:       NilError,
			ContractAddress: common.Address{},
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
		},
	}
	deployFromFactoryTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
}

func TestEVMTransfers(t *testing.T) {
	require := require.New(t)

	testCtx := NewTestContext()
	height := uint64(0)
	to := storage.ToEVMAddress(testCtx.Recipient)

	deployTest := &chaintest.ActionTest{
		Name: "deploy contract for transfer tests",
		Action: &EvmCall{
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     testCtx.TestContractABI.Bytecode,
			Keys:     state.Keys{},
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockContext(height, testCtx.Timestamp),
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:         true,
			UsedGas:         0x8459a,
			Return:          testCtx.TestContractABI.DeployedBytecode,
			ErrorCode:       NilError,
			ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
		},
	}
	deployTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))

	contractAddr := crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce)
	testCtx.Nonce++

	directTransfer := &chaintest.ActionTest{
		Name: "direct EOA to EOA transfer",
		Action: &EvmCall{
			To:             to,
			NotNullAddress: true,
			Value:          1,
			GasLimit:       testCtx.SufficientGas,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockContext(height, testCtx.Timestamp),
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:   true,
			UsedGas:   0x5208,
			Return:    nil,
			ErrorCode: NilError,
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			recipientAccount, err := storage.GetAccount(ctx, mu, to)
			require.NoError(err)
			decodedAccount, err := storage.DecodeAccount(recipientAccount)
			require.NoError(err)
			require.Equal(uint256.NewInt(1), decodedAccount.Balance)
		},
	}
	directTransfer.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))

	transferData := testCtx.TestContractABI.ABI.Methods["transferToAddress"].ID
	transferData = append(transferData, common.LeftPadBytes(to.Bytes(), 32)...)

	transferToAddress := &chaintest.ActionTest{
		Name: "transfer through transferToAddress",
		Action: &EvmCall{
			To:             contractAddr,
			NotNullAddress: true,
			Value:          1,
			GasLimit:       testCtx.SufficientGas,
			Data:           transferData,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockContext(height, testCtx.Timestamp),
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:   true,
			UsedGas:   0x7a38,
			Return:    nil,
			ErrorCode: NilError,
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			recipientAccount, err := storage.GetAccount(ctx, mu, to)
			require.NoError(err)
			decodedAccount, err := storage.DecodeAccount(recipientAccount)
			require.NoError(err)
			require.Equal(uint256.NewInt(2), decodedAccount.Balance) // Now has 2 (1 from previous + 1 from this transfer)
		},
	}
	transferToAddress.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))

	transferThroughData := testCtx.TestContractABI.ABI.Methods["transferThroughContract"].ID
	transferThroughData = append(transferThroughData, common.LeftPadBytes(to.Bytes(), 32)...)

	transferThroughContract := &chaintest.ActionTest{
		Name: "transfer through transferThroughContract",
		Action: &EvmCall{
			To:             contractAddr,
			NotNullAddress: true,
			Value:          1,
			GasLimit:       testCtx.SufficientGas,
			Data:           transferThroughData,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockContext(height, testCtx.Timestamp),
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:   true,
			UsedGas:   0x8073,
			Return:    nil,
			ErrorCode: NilError,
		},
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			recipientAccount, err := storage.GetAccount(ctx, mu, to)
			require.NoError(err)
			decodedAccount, err := storage.DecodeAccount(recipientAccount)
			require.NoError(err)
			require.Equal(uint256.NewInt(3), decodedAccount.Balance) // Now has 3 (2 from previous + 1 from this transfer)

			// Contract balance should be 0 as it forwards all received tokens
			contractAccount, err := storage.GetAccount(ctx, mu, contractAddr)
			require.NoError(err)
			decodedContractAccount, err := storage.DecodeAccount(contractAccount)
			require.NoError(err)
			require.Equal(uint256.NewInt(0), decodedContractAccount.Balance)
		},
	}
	transferThroughContract.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
}

func TestEVMTstate(t *testing.T) {
	require := require.New(t)

	testCtx := NewTestContext()
	height := uint64(0)

	// First deploy the test contract
	deployTest := &chaintest.ActionTest{
		Name: "deploy contract for calls",
		Action: &EvmCall{
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     testCtx.TestContractABI.Bytecode,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockContext(height, testCtx.Timestamp),
		Actor:    testCtx.From,
		ActionID: testCtx.ActionID,
		ExpectedOutputs: &EvmCallResult{
			Success:         true,
			UsedGas:         0x8459a,
			Return:          testCtx.TestContractABI.DeployedBytecode,
			ErrorCode:       NilError,
			ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
		},
	}
	deployTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))

	contractAddr := crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce)
	testCtx.Nonce++

	value := big.NewInt(42)
	setValueData := testCtx.TestContractABI.ABI.Methods["setValue"].ID
	setValueData = append(setValueData, common.LeftPadBytes(value.Bytes(), 32)...)

	call := &EvmCall{
		To:             contractAddr,
		NotNullAddress: true,
		Value:          0,
		GasLimit:       testCtx.SufficientGas,
		Data:           setValueData,
		Keys:           state.Keys{},
	}

	tstateTest := &chaintest.ActionTest{
		Name:        "incorrect state keys should revert",
		Action:      call,
		Rules:       testCtx.Rules,
		State:       testCtx.State,
		BlockCtx:    chain.NewBlockContext(height, testCtx.Timestamp),
		Actor:       testCtx.From,
		ActionID:    testCtx.ActionID,
		ExpectedErr: tstate.ErrInvalidKeyOrPermission,
	}

	sk := state.SimulatedKeys{}
	ts := tstate.New(0)
	tsv := ts.NewView(sk, testCtx.State, 0)
	result, err := tstateTest.Action.Execute(testCtx.Context, chain.NewBlockContext(height, testCtx.Timestamp), testCtx.Rules, tsv, testCtx.From, testCtx.ActionID)
	require.NoError(err)
	require.Equal(result.(*EvmCallResult).ErrorCode, NilError)
	call.Keys = sk.StateKeys()

	stateKeys := call.StateKeys(testCtx.From, testCtx.ActionID)

	wrongKeys := state.Keys{
		"wrongKey": state.All,
	}

	storage := make(map[string][]byte, len(stateKeys))
	for key := range stateKeys {
		val, err := testCtx.State.GetValue(testCtx.Context, []byte(key))
		if errors.Is(err, database.ErrNotFound) {
			continue
		}
		require.NoError(err)
		storage[key] = val
	}
	ts = tstate.New(0)
	tsv = ts.NewView(wrongKeys, state.ImmutableStorage(storage), 0)

	tstateTest.State = tsv

	tstateTest.Run(testCtx.Context, t)

	tsv = ts.NewView(stateKeys, state.ImmutableStorage(storage), 0)
	tstateTest.State = tsv
	tstateTest.Name = "correct state keys should succeed"
	tstateTest.ExpectedErr = nil
	tstateTest.ExpectedOutputs = &EvmCallResult{
		Success:   true,
		UsedGas:   0xaf73,
		Return:    []uint8(nil),
		ErrorCode: NilError,
	}
	tstateTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
}
