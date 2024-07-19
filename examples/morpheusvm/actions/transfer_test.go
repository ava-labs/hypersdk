// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

func TestTransferAction(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)
	emptyBalanceKey := storage.BalanceKey(codec.EmptyAddress)
	addrSlice := make([]byte, codec.AddressLen)
	for i := range addrSlice {
		addrSlice[i] = 1
	}
	oneAddr, err := codec.ToAddress(addrSlice)
	req.NoError(err)

	tests := []chaintest.ActionTest{
		{
			Name:  "ZeroTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
		{
			Name:  "InvalidStateKey",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State:       ts.NewView(make(state.Keys), map[string][]byte{}),
			ExpectedErr: tstate.ErrInvalidKeyOrPermission,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				keys.Add(string(emptyBalanceKey), state.Read)
				tsv := ts.NewView(keys, map[string][]byte{})
				return tsv
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SelfTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				return ts.NewView(keys, store.Storage)
			}(),
			Assertion: func(t *testing.T, store state.Mutable) {
				require := require.New(t)
				balance, err := storage.GetBalance(context.Background(), store, codec.EmptyAddress)
				require.NoError(err)
				require.Equal(balance, uint64(1))
			},
		},
		{
			Name:  "OverflowBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: math.MaxUint64,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				return ts.NewView(keys, store.Storage)
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SimpleTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    oneAddr,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				keys.Add(string(storage.BalanceKey(oneAddr)), state.All)
				return ts.NewView(keys, store.Storage)
			}(),
			Assertion: func(t *testing.T, store state.Mutable) {
				require := require.New(t)
				receiverBalance, err := storage.GetBalance(context.Background(), store, oneAddr)
				require.NoError(err)
				senderBalance, err := storage.GetBalance(context.Background(), store, codec.EmptyAddress)
				require.NoError(err)
				require.Equal(receiverBalance, uint64(1))
				require.Equal(senderBalance, uint64(0))
			},
		},
	}

	chaintest.Run(context.Background(), t, tests)
}
