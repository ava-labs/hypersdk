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
