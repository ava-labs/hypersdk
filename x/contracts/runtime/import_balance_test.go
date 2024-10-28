// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestImportBalanceSendBalanceToAnotherContract(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("balance")
	require.NoError(err)

	r := contract.Runtime
	stateManager := r.StateManager.(TestStateManager)
	stateManager.Balances[contract.Address] = 3

	// create a new instance of the balance contract
	newInstanceAddress := codec.CreateAddress(0, ids.GenerateTestID())
	contractID, err := stateManager.GetAccountContract(ctx, contract.Address)
	require.NoError(err)
	require.NoError(r.StateManager.SetAccountContract(ctx, newInstanceAddress, contractID))
	stateManager.Balances[newInstanceAddress] = 0

	// contract 2 starts with 0 balance
	result, err := r.CallContract(newInstanceAddress, "balance", nil)
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	// send 2 from contract1 to contract2, results in 1 being returned since that is the new balance of contract 1
	result, err = contract.Call("send_via_call", newInstanceAddress, uint64(1000000), uint64(2))
	require.NoError(err)
	require.Equal(uint64(1), into[uint64](result))

	// contract 2 should now have 2 balance
	result, err = contract.WithActor(newInstanceAddress).Call("balance")
	require.NoError(err)
	require.Equal(uint64(2), into[uint64](result))
}

func TestImportBalanceGetBalance(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	actor := codec.CreateAddress(0, ids.GenerateTestID())
	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("balance")
	require.NoError(err)
	contract.Runtime.StateManager.(TestStateManager).Balances[actor] = 3
	result, err := contract.WithActor(actor).Call("balance")
	require.NoError(err)
	require.Equal(uint64(3), into[uint64](result))
}

func TestImportBalanceSend(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	actor := codec.CreateAddress(0, ids.GenerateTestID())
	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("balance")
	require.NoError(err)

	contract.Runtime.StateManager.(TestStateManager).Balances[contract.Address] = 3
	result, err := contract.Call("send_balance", actor)
	require.NoError(err)
	require.True(into[bool](result))

	result, err = contract.WithActor(actor).Call("balance")
	require.NoError(err)
	require.Equal(uint64(1), into[uint64](result))

	result, err = contract.WithActor(contract.Address).Call("balance")
	require.NoError(err)
	require.Equal(uint64(2), into[uint64](result))
}
