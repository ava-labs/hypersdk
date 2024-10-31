// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v25"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/contracts/test"
)

func TestCallContext(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	contractID := ids.GenerateTestID()
	contractAccount := codec.CreateAddress(0, contractID)
	stringedID := string(contractID[:])
	contractManager := NewContractStateManager(test.NewTestDB(), []byte{})
	err := contractManager.SetAccountContract(ctx, contractAccount, ContractID(stringedID))
	require.NoError(err)
	testStateManager := &TestStateManager{
		ContractManager: contractManager,
	}

	err = testStateManager.CompileAndSetContract(ContractID(stringedID), "call_contract")
	require.NoError(err)

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
	).WithDefaults(
		CallInfo{
			State:    testStateManager,
			Contract: contractAccount,
			Fuel:     1000000,
		})
	actor := codec.CreateAddress(1, ids.GenerateTestID())

	result, err := r.WithActor(actor).CallContract(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.NoError(err)
	require.Equal(actor, into[codec.Address](result))

	result, err = r.WithActor(codec.CreateAddress(2, ids.GenerateTestID())).CallContract(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.NoError(err)
	require.NotEqual(actor, into[codec.Address](result))

	result, err = r.WithFuel(0).CallContract(
		ctx,
		&CallInfo{
			FunctionName: "actor_check",
		})
	require.Equal(wasmtime.OutOfFuel, *err.(*wasmtime.Trap).Code())
	require.Nil(result)
}

func TestCallContextPreventOverwrite(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	contract0ID := ids.GenerateTestID()
	contract0Address := codec.CreateAddress(0, contract0ID)
	contract1ID := ids.GenerateTestID()
	contract1Address := codec.CreateAddress(1, contract1ID)
	stringedID0 := string(contract0ID[:])

	contractManager := NewContractStateManager(test.NewTestDB(), []byte{})
	err := contractManager.SetAccountContract(ctx, contract0Address, ContractID(stringedID0))
	require.NoError(err)
	testStateManager := &TestStateManager{
		ContractManager: contractManager,
	}
	err = testStateManager.CompileAndSetContract(ContractID(stringedID0), "call_contract")
	require.NoError(err)

	r := NewRuntime(
		NewConfig(),
		logging.NoLog{},
	).WithDefaults(
		CallInfo{
			Contract: contract0Address,
			State:    testStateManager,
			Fuel:     1000000,
		})

	stringedID1 := string(contract1ID[:])
	contractManager1 := NewContractStateManager(test.NewTestDB(), []byte{})
	err = contractManager.SetAccountContract(ctx, contract1Address, ContractID(stringedID1))
	require.NoError(err)
	testStateManager1 := &TestStateManager{
		ContractManager: contractManager1,
	}

	err = testStateManager1.CompileAndSetContract(ContractID(stringedID1), "call_contract")
	require.NoError(err)

	// try to use a context that has a default contract with a different contract
	result, err := r.CallContract(
		ctx,
		&CallInfo{
			Contract:     contract1Address,
			State:        testStateManager1,
			FunctionName: "actor_check",
		})
	require.ErrorIs(err, errCannotOverwrite)
	require.Nil(result)
}
