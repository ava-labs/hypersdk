// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

func TestTransferAction(t *testing.T) {
	emptyBalanceKey := storage.BalanceKey(codec.EmptyAddress)
	addr, err := codectest.NewRandomAddress()
	require.NoError(t, err)

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
			Name:  "InvalidAddress",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State:       chaintest.NewInMemoryStore(),
			ExpectedErr: storage.ErrInvalidAddress,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			State: func() state.Mutable {
				s := chaintest.NewInMemoryStore()
				require.NoError(t, storage.AddBalance(
					context.Background(),
					s,
					codec.EmptyAddress,
					0,
					true,
				))
				return s
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
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				balance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, balance, uint64(1))
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
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				return store
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SimpleTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    addr,
				Value: 1,
			},
			State: func() state.Mutable {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				keys.Add(string(storage.BalanceKey(addr)), state.All)
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				receiverBalance, err := storage.GetBalance(ctx, store, addr)
				require.NoError(t, err)
				require.Equal(t, receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, senderBalance, uint64(0))
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func BenchmarkSimpleTransfer(b *testing.B) {
	require := require.New(b)
	to := codec.CreateAddress(0, ids.GenerateTestID())
	from := codec.CreateAddress(0, ids.GenerateTestID())
	toBalanceKey := storage.BalanceKey(to)
	fromBalanceKey := storage.BalanceKey(from)

	transferActionTest := &chaintest.ActionBenchmark{
		Name:  "SimpleTransferBenchmark",
		Actor: from,
		Action: &Transfer{
			To:    to,
			Value: 1,
		},
		CreateState: func() state.Mutable {
			keys := make(state.Keys)
			store := chaintest.NewInMemoryStore()
			err := storage.SetBalance(context.Background(), store, from, 1)
			require.NoError(err)
			keys.Add(string(toBalanceKey), state.All)
			keys.Add(string(fromBalanceKey), state.All)
			return store
		},
		Assertion: func(ctx context.Context, b *testing.B, store state.Mutable) {
			toBalance, err := storage.GetBalance(ctx, store, to)
			require.NoError(err)
			require.Equal(uint64(1), toBalance)

			fromBalance, err := storage.GetBalance(ctx, store, from)
			require.NoError(err)
			require.Equal(uint64(0), fromBalance)
		},
	}

	ctx := context.Background()
	transferActionTest.Run(ctx, b)
}
