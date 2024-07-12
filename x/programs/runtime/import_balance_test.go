// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/test"
)

func TestImportBalanceSendBalanceToAnotherProgram(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	program := newTestProgram(ctx, "balance")
	r := program.Runtime
	stateManager := r.StateManager.(test.StateManager)
	stateManager.Balances[program.Address] = 3

	// create a new instance of the balance program
	newInstanceAddress := codec.CreateAddress(0, ids.GenerateTestID())
	programID, err := stateManager.GetAccountProgram(ctx, program.Address)
	require.NoError(err)
	r.StateManager.SetAccountProgram(ctx, newInstanceAddress, programID)
	stateManager.Balances[newInstanceAddress] = 0

	// program 2 starts with 0 balance
	result, err := r.CallProgram(newInstanceAddress, "balance")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	// send 2 from program1 to program2, results in 1 being returned since that is the new balance of program 1
	result, err = program.Call("send_via_call", newInstanceAddress, uint64(1000000), uint64(2))
	require.NoError(err)
	require.Equal(uint64(1), into[uint64](result))

	// program 2 should now have 2 balance
	result, err = program.WithActor(newInstanceAddress).Call("balance")
	require.NoError(err)
	require.Equal(uint64(2), into[uint64](result))
}

func TestImportBalanceGetBalance(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	actor := codec.CreateAddress(0, ids.GenerateTestID())
	program := newTestProgram(ctx, "balance")
	program.Runtime.StateManager.(test.StateManager).Balances[actor] = 3
	result, err := program.WithActor(actor).Call("balance")
	require.NoError(err)
	require.Equal(uint64(3), into[uint64](result))
}

func TestImportBalanceSend(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	actor := codec.CreateAddress(0, ids.GenerateTestID())
	program := newTestProgram(ctx, "balance")
	program.Runtime.StateManager.(test.StateManager).Balances[program.Address] = 3
	result, err := program.Call("send_balance", actor)
	require.NoError(err)
	require.True(into[bool](result))

	result, err = program.WithActor(actor).Call("balance")
	require.NoError(err)
	require.Equal(uint64(1), into[uint64](result))

	result, err = program.WithActor(program.Address).Call("balance")
	require.NoError(err)
	require.Equal(uint64(2), into[uint64](result))
}
