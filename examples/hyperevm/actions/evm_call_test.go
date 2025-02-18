// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/state"
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
		To:       common.Address{1},
		Value:    1,
		GasLimit: 1000000,
		Data:     []byte{},
	}
	msg = evmCall.toMessage(sender)
	require.NotNil(msg.To)

	t.Log("testing serialization - value")
	evmCall = &EvmCall{
		To:       common.Address{1},
		Value:    10,
		GasLimit: 1000000,
		Data:     []byte{},
	}
	msg = evmCall.toMessage(sender)
	require.IsType(msg.Value, &big.Int{})
}

func TestDeployment(t *testing.T) {
	require := require.New(t)

	testCtx := NewTestContext()
	blockContext := chain.NewBlockCtx(0, testCtx.Timestamp)

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
			To:       factoryAddr,
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     deployData,
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
		BlockCtx: chain.NewBlockCtx(height, testCtx.Timestamp),
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
			To:       to,
			Value:    1,
			GasLimit: testCtx.SufficientGas,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockCtx(height, testCtx.Timestamp),
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
			To:       contractAddr,
			Value:    1,
			GasLimit: testCtx.SufficientGas,
			Data:     transferData,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockCtx(height, testCtx.Timestamp),
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
			To:       contractAddr,
			Value:    1,
			GasLimit: testCtx.SufficientGas,
			Data:     transferThroughData,
		},
		Rules:    testCtx.Rules,
		State:    testCtx.State,
		BlockCtx: chain.NewBlockCtx(height, testCtx.Timestamp),
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
