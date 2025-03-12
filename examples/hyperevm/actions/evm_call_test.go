// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestSerialization(t *testing.T) {
	require := require.New(t)

	sender := common.Address{1}
	t.Log("testing serialization - empty To")
	evmCall := &EvmCall{
		To:            common.Address{},
		IsNullAddress: true,
		Value:         1,
		GasLimit:      1000000,
		Data:          []byte{},
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
	r := require.New(t)

	testCtx, err := NewTestContext()
	r.NoError(err)
	actionCtx := chain.NewActionContext(0, testCtx.Timestamp, ids.Empty)

	testContractABI, ok := testCtx.ABIs["TestContract"]
	r.True(ok)
	factoryContractABI, ok := testCtx.ABIs["ContractFactory"]
	r.True(ok)

	expectedOutput := &EvmActionResult{
		Success:         true,
		UsedGas:         0x8459a,
		Return:          testContractABI.DeployedBytecode,
		ErrorCode:       NilError,
		ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
	}

	firstDeployTest := &chaintest.ActionTest{
		Name: "deploy contract",
		Action: &EvmCall{
			Value:         0,
			IsNullAddress: true,
			GasLimit:      testCtx.SufficientGas,
			Data:          testContractABI.Bytecode,
			Keys:          state.Keys{},
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       actionCtx,
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			r := require.New(t)
			code, err := storage.GetCode(ctx, mu, crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce))
			r.NoError(err)
			r.NotEmpty(code)
			r.ElementsMatch(code, testContractABI.DeployedBytecode)
		},
	}
	firstDeployTest.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))
	testCtx.Nonce++

	expectedOutput = &EvmActionResult{
		Success:         true,
		UsedGas:         0x8459a,
		Return:          testContractABI.DeployedBytecode,
		ErrorCode:       NilError,
		ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
	}

	secondDeployTest := &chaintest.ActionTest{
		Name: "deploy same contract again",
		Action: &EvmCall{
			IsNullAddress: true,
			Value:         0,
			GasLimit:      testCtx.SufficientGas,
			Data:          testContractABI.Bytecode,
			From:          storage.ToEVMAddress(testCtx.From),
		},
		ActionCtx:       actionCtx,
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			r := require.New(t)
			code, err := storage.GetCode(ctx, mu, crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce))
			r.NoError(err)
			r.NotEmpty(code)
			r.ElementsMatch(code, testContractABI.DeployedBytecode)
		},
	}
	secondDeployTest.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))
	testCtx.Nonce++

	expectedOutput = &EvmActionResult{
		Success:         true,
		UsedGas:         0x98bf6,
		Return:          factoryContractABI.DeployedBytecode,
		ErrorCode:       NilError,
		ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
	}

	factoryDeployTest := &chaintest.ActionTest{
		Name: "deploy factory contract",
		Action: &EvmCall{
			IsNullAddress: true,
			Value:         0,
			GasLimit:      testCtx.SufficientGas,
			Data:          factoryContractABI.Bytecode,
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       actionCtx,
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			r := require.New(t)
			code, err := storage.GetCode(ctx, mu, crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce))
			r.NoError(err)
			r.NotEmpty(code)
			r.ElementsMatch(code, factoryContractABI.DeployedBytecode)
		},
	}
	factoryDeployTest.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))
	testCtx.Nonce++

	factoryAddr := crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce) // we just deployed the factory

	expectedOutput = &EvmActionResult{
		Success:         true,
		From:            storage.ToEVMAddress(testCtx.From),
		To:              factoryAddr,
		UsedGas:         0x5248,
		Return:          nil,
		ErrorCode:       NilError,
		ContractAddress: common.Address{},
	}

	deployData := factoryContractABI.ABI.Methods["deployContract"].ID
	deployFromFactoryTest := &chaintest.ActionTest{
		Name: "deploy contract from a contract",
		Action: &EvmCall{
			To:       factoryAddr,
			Value:    0,
			GasLimit: testCtx.SufficientGas,
			Data:     deployData,
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       actionCtx,
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
	}
	deployFromFactoryTest.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))
}

func TestEVMTransfers(t *testing.T) {
	r := require.New(t)

	testCtx, err := NewTestContext()
	r.NoError(err)
	to := storage.ToEVMAddress(testCtx.Recipient)

	testContractABI, ok := testCtx.ABIs["TestContract"]
	r.True(ok)

	expectedOutput := &EvmActionResult{
		Success:         true,
		UsedGas:         0x8459a,
		Return:          testContractABI.DeployedBytecode,
		ErrorCode:       NilError,
		ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
	}

	deployTest := &chaintest.ActionTest{
		Name: "deploy contract for transfer tests",
		Action: &EvmCall{
			IsNullAddress: true,
			Value:         0,
			GasLimit:      testCtx.SufficientGas,
			Data:          testContractABI.Bytecode,
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       chain.NewActionContext(0, testCtx.Timestamp, ids.Empty),
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
	}
	deployTest.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))

	contractAddr := crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce)
	testCtx.Nonce++

	expectedOutput = &EvmActionResult{
		Success:   true,
		UsedGas:   0x5208,
		Return:    nil,
		ErrorCode: NilError,
		From:      storage.ToEVMAddress(testCtx.From),
		To:        to,
	}

	directTransfer := &chaintest.ActionTest{
		Name: "direct EOA to EOA transfer",
		Action: &EvmCall{
			To:       to,
			Value:    1,
			GasLimit: testCtx.SufficientGas,
			From:     storage.ToEVMAddress(testCtx.From),
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       chain.NewActionContext(0, testCtx.Timestamp, ids.Empty),
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			r := require.New(t)
			recipientAccount, err := storage.GetAccount(ctx, mu, to)
			r.NoError(err)
			decodedAccount, err := storage.DecodeAccount(recipientAccount)
			r.NoError(err)
			r.Equal(uint256.NewInt(1), decodedAccount.Balance)
		},
	}
	directTransfer.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))

	transferData := testContractABI.ABI.Methods["transferToAddress"].ID
	transferData = append(transferData, common.LeftPadBytes(to.Bytes(), 32)...)

	expectedOutput = &EvmActionResult{
		Success:   true,
		UsedGas:   0x7a38,
		Return:    nil,
		ErrorCode: NilError,
		To:        contractAddr,
		From:      storage.ToEVMAddress(testCtx.From),
	}

	transferToAddress := &chaintest.ActionTest{
		Name: "transfer through transferToAddress",
		Action: &EvmCall{
			To:       contractAddr,
			From:     storage.ToEVMAddress(testCtx.From),
			Value:    1,
			GasLimit: testCtx.SufficientGas,
			Data:     transferData,
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       chain.NewActionContext(0, testCtx.Timestamp, ids.Empty),
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			r := require.New(t)
			recipientAccount, err := storage.GetAccount(ctx, mu, to)
			r.NoError(err)
			decodedAccount, err := storage.DecodeAccount(recipientAccount)
			r.NoError(err)
			r.Equal(uint256.NewInt(2), decodedAccount.Balance) // Now has 2 (1 from previous + 1 from this transfer)
		},
	}
	transferToAddress.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))

	transferThroughData := testContractABI.ABI.Methods["transferThroughContract"].ID
	transferThroughData = append(transferThroughData, common.LeftPadBytes(to.Bytes(), 32)...)

	action := &EvmCall{
		To:       contractAddr,
		Value:    1,
		GasLimit: testCtx.SufficientGas,
		Data:     transferThroughData,
	}

	ts := tstate.New(0)
	tsv := ts.NewView(state.CompletePermissions, testCtx.State, 0)
	output, err := action.Execute(testCtx.Context, chain.NewActionContext(0, testCtx.Timestamp, ids.Empty), testCtx.Rules, tsv, testCtx.From, testCtx.ActionID)
	r.NoError(err)

	unmarshaledResult, err := UnmarshalEvmActionResult(output)
	r.NoError(err)

	typedResult, ok := unmarshaledResult.(*EvmActionResult)
	r.True(ok)

	expectedOutput = &EvmActionResult{
		Success:   true,
		UsedGas:   0x8073,
		To:        contractAddr,
		From:      storage.ToEVMAddress(testCtx.From),
		Return:    nil,
		ErrorCode: NilError,
		Logs:      typedResult.Logs,
	}

	transferThroughContract := &chaintest.ActionTest{
		Name:            "transfer through transferThroughContract",
		Action:          action,
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       chain.NewActionContext(0, testCtx.Timestamp, ids.Empty),
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
		Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
			r := require.New(t)
			recipientAccount, err := storage.GetAccount(ctx, mu, to)
			r.NoError(err)
			decodedAccount, err := storage.DecodeAccount(recipientAccount)
			r.NoError(err)
			r.Equal(uint256.NewInt(3), decodedAccount.Balance) // Now has 3 (2 from previous + 1 from this transfer)

			// Contract balance should be 0 as it forwards all received tokens
			contractAccount, err := storage.GetAccount(ctx, mu, contractAddr)
			r.NoError(err)
			decodedContractAccount, err := storage.DecodeAccount(contractAccount)
			r.NoError(err)
			r.Equal(uint256.NewInt(0), decodedContractAccount.Balance)
		},
	}
	transferThroughContract.Run(testCtx.Context, t)
	r.NoError(testCtx.State.Commit(testCtx.Context))
}

func TestEVMTstate(t *testing.T) {
	require := require.New(t)

	testCtx, err := NewTestContext()
	require.NoError(err)

	testContractABI, ok := testCtx.ABIs["TestContract"]
	require.True(ok)

	expectedOutput := &EvmActionResult{
		Success:         true,
		From:            storage.ToEVMAddress(testCtx.From),
		UsedGas:         0x8459a,
		Return:          testContractABI.DeployedBytecode,
		ErrorCode:       NilError,
		ContractAddress: crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce),
	}

	// First deploy the test contract
	deployTest := &chaintest.ActionTest{
		Name: "deploy contract for calls",
		Action: &EvmCall{
			IsNullAddress: true,
			Value:         0,
			GasLimit:      testCtx.SufficientGas,
			Data:          testContractABI.Bytecode,
		},
		Rules:           testCtx.Rules,
		State:           testCtx.State,
		ActionCtx:       chain.NewActionContext(0, testCtx.Timestamp, ids.Empty),
		Actor:           testCtx.From,
		ActionID:        testCtx.ActionID,
		ExpectedOutputs: expectedOutput.Bytes(),
	}
	deployTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))

	contractAddr := crypto.CreateAddress(storage.ToEVMAddress(testCtx.From), testCtx.Nonce)
	testCtx.Nonce++

	value := big.NewInt(42)
	setValueData := testContractABI.ABI.Methods["setValue"].ID
	setValueData = append(setValueData, common.LeftPadBytes(value.Bytes(), 32)...)

	call := &EvmCall{
		To:       contractAddr,
		Value:    0,
		GasLimit: testCtx.SufficientGas,
		Data:     setValueData,
		Keys:     state.Keys{},
	}

	actionCtx := chain.NewActionContext(0, testCtx.Timestamp, ids.Empty)

	tstateTest := &chaintest.ActionTest{
		Name:        "incorrect state keys should revert",
		Action:      call,
		Rules:       testCtx.Rules,
		State:       testCtx.State,
		ActionCtx:   actionCtx,
		Actor:       testCtx.From,
		ActionID:    testCtx.ActionID,
		ExpectedErr: tstate.ErrInvalidKeyOrPermission,
	}

	sk := state.SimulatedKeys{}
	ts := tstate.New(0)
	tsv := ts.NewView(sk, testCtx.State, 0)
	result, err := tstateTest.Action.Execute(testCtx.Context, actionCtx, testCtx.Rules, tsv, testCtx.From, testCtx.ActionID)
	require.NoError(err)

	unmarshaledResult, err := UnmarshalEvmActionResult(result)
	require.NoError(err)

	typedResult, ok := unmarshaledResult.(*EvmActionResult)
	require.True(ok)

	require.Equal(typedResult.ErrorCode, NilError)
	call.Keys = sk.StateKeys()

	stateKeys := call.StateKeys(testCtx.From, testCtx.ActionID)

	wrongKeys := state.Keys{
		"wrongKey": state.All,
	}

	st := make(map[string][]byte, len(stateKeys))
	for key := range stateKeys {
		val, err := testCtx.State.GetValue(testCtx.Context, []byte(key))
		if errors.Is(err, database.ErrNotFound) {
			continue
		}
		require.NoError(err)
		st[key] = val
	}
	ts = tstate.New(0)
	tsv = ts.NewView(wrongKeys, state.ImmutableStorage(st), 0)

	tstateTest.State = tsv

	tstateTest.Run(testCtx.Context, t)

	tsvTemp := ts.NewView(state.CompletePermissions, state.ImmutableStorage(st), 0)
	output, err := tstateTest.Action.Execute(testCtx.Context, actionCtx, testCtx.Rules, tsvTemp, testCtx.From, testCtx.ActionID)
	require.NoError(err)

	unmarshaledResult, err = UnmarshalEvmActionResult(output)
	require.NoError(err)

	typedResult, ok = unmarshaledResult.(*EvmActionResult)
	require.True(ok)

	expectedOutput = &EvmActionResult{
		Success:   true,
		To:        contractAddr,
		From:      storage.ToEVMAddress(testCtx.From),
		UsedGas:   0xaf73,
		Return:    []uint8(nil),
		ErrorCode: NilError,
		Logs:      typedResult.Logs,
	}

	tsv = ts.NewView(stateKeys, state.ImmutableStorage(st), 0)
	tstateTest.State = tsv
	tstateTest.Name = "correct state keys should succeed"
	tstateTest.ExpectedErr = nil
	tstateTest.ExpectedOutputs = expectedOutput.Bytes()
	tstateTest.Run(testCtx.Context, t)
	require.NoError(testCtx.State.Commit(testCtx.Context))
}

func TestEVMLogs(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	testCtx, err := NewTestContext()
	r.NoError(err)
	height := uint64(0)
	actionCtx := chain.NewActionContext(height, testCtx.Timestamp, ids.Empty)

	eventContractABI, ok := testCtx.ABIs["EmitEvent"]
	r.True(ok)

	// Deploy event contract
	action := &EvmCall{
		IsNullAddress: true,
		Value:         0,
		GasLimit:      testCtx.SufficientGas,
		Data:          eventContractABI.Bytecode,
		From:          storage.ToEVMAddress(testCtx.From),
	}

	result, err := action.Execute(ctx, actionCtx, testCtx.Rules, testCtx.State, codec.EmptyAddress, ids.Empty)
	r.NoError(err)

	unmarshaledResult, err := UnmarshalEvmActionResult(result)
	r.NoError(err)

	deployResult, ok := unmarshaledResult.(*EvmActionResult)
	r.True(ok)
	r.True(deployResult.Success)
	r.Equal(eventContractABI.DeployedBytecode, deployResult.Return)
	r.Equal(NilError, deployResult.ErrorCode)

	eventContractAddress := deployResult.ContractAddress
	emitData := eventContractABI.ABI.Methods["EmitEvent"].ID

	emitAction := &EvmCall{
		To:       eventContractAddress,
		Value:    0,
		GasLimit: testCtx.SufficientGas,
		Data:     emitData,
		From:     storage.ToEVMAddress(testCtx.From),
	}

	result, err = emitAction.Execute(ctx, actionCtx, testCtx.Rules, testCtx.State, codec.EmptyAddress, ids.Empty)
	r.NoError(err)

	unmarshaledResult, err = UnmarshalEvmActionResult(result)
	r.NoError(err)

	emitResult, ok := unmarshaledResult.(*EvmActionResult)
	r.True(ok)
	r.True(emitResult.Success)

	r.Len(emitResult.Logs, 1)
	log := emitResult.Logs[0]
	r.Equal(eventContractAddress, log.Address)
}
