// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestCallContext(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	contractID := ids.GenerateTestID()
	contractAccount := codec.CreateAddress(0, contractID)
	stringedID := string(contractID[:])
	testStateManager := &TestStateManager{
		ContractsMap: map[string][]byte{},
		AccountMap:   map[codec.Address]string{contractAccount: stringedID},
	}
	err := testStateManager.CompileAndSetContract(ContractID(stringedID), "call_contract")
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

	testStateManager := &TestStateManager{
		ContractsMap: map[string][]byte{},
		AccountMap:   map[codec.Address]string{contract0Address: stringedID0},
	}

	err := testStateManager.CompileAndSetContract(ContractID(stringedID0), "call_contract")
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
	testStateManager1 := &TestStateManager{
		ContractsMap: map[string][]byte{},
		AccountMap:   map[codec.Address]string{contract1Address: stringedID1},
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
