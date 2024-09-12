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

func BenchmarkRuntimeCallContractBasic(b *testing.B) {
	require := require.New(b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	for i := 0; i < b.N; i++ {
		result, err := contract.Call("get_value")
		require.NoError(err)
		require.Equal(uint64(0), into[uint64](result))
	}
}

func TestRuntimeCallContractBasicAttachValue(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	actor := codec.CreateAddress(0, ids.GenerateTestID())
	contract.Runtime.StateManager.(TestStateManager).Balances[actor] = 10

	actorBalance, err := contract.Runtime.StateManager.GetBalance(context.Background(), actor)
	require.NoError(err)
	require.Equal(uint64(10), actorBalance)

	// calling a contract with a value transfers that amount from the caller to the contract
	result, err := contract.WithActor(actor).WithValue(4).Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))

	actorBalance, err = contract.Runtime.StateManager.GetBalance(context.Background(), actor)
	require.NoError(err)
	require.Equal(uint64(6), actorBalance)

	contractBalance, err := contract.Runtime.StateManager.GetBalance(context.Background(), contract.Address)
	require.NoError(err)
	require.Equal(uint64(4), contractBalance)
}

func TestRuntimeCallContractBasic(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("simple")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(uint64(0), into[uint64](result))
}

type ComplexReturn struct {
	Contract codec.Address
	MaxUnits uint64
}

func TestRuntimeCallContractComplexReturn(t *testing.T) {
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt := newTestRuntime(ctx)
	contract, err := rt.newTestContract("return_complex_type")
	require.NoError(err)

	result, err := contract.Call("get_value")
	require.NoError(err)
	require.Equal(ComplexReturn{Contract: contract.Address, MaxUnits: 1000}, into[ComplexReturn](result))
}
