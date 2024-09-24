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
)

func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()

	tests := []chaintest.ActionTest[*chaintest.InMemoryStore]{
		{
			Name:  "ZeroTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer[*chaintest.InMemoryStore]{
				To:    codec.EmptyAddress,
				Value: 0,
			},
			ExpectedErr: ErrOutputValueZero,
		},
		{
			Name:  "InvalidAddress",
			Actor: codec.EmptyAddress,
			Action: &Transfer[*chaintest.InMemoryStore]{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			View:        chaintest.NewInMemoryStore(),
			ExpectedErr: storage.ErrInvalidAddress,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer[*chaintest.InMemoryStore]{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			View: func() *chaintest.InMemoryStore {
				state := chaintest.NewInMemoryStore()

				_, err := storage.AddBalance(
					context.Background(),
					state,
					codec.EmptyAddress,
					0,
					true,
				)
				require.NoError(t, err)
				return state
			}(),

			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SelfTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer[*chaintest.InMemoryStore]{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			View: func() *chaintest.InMemoryStore {
				state := chaintest.NewInMemoryStore()

				require.NoError(t, storage.SetBalance(context.Background(), state, codec.EmptyAddress, 1))
				return state
			}(),
			Assertion: func(ctx context.Context, t *testing.T, state *chaintest.InMemoryStore) {
				balance, err := storage.GetBalance(ctx, state, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, balance, uint64(1))
			},
			ExpectedOutputs: &TransferResult{
				SenderBalance:   0,
				ReceiverBalance: 1,
			},
		},
		{
			Name:  "OverflowBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer[*chaintest.InMemoryStore]{
				To:    codec.EmptyAddress,
				Value: math.MaxUint64,
			},
			View: func() *chaintest.InMemoryStore {
				state := chaintest.NewInMemoryStore()

				require.NoError(t, storage.SetBalance(context.Background(), state, codec.EmptyAddress, 1))
				return state
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
		},
		{
			Name:  "SimpleTransfer",
			Actor: codec.EmptyAddress,
			Action: &Transfer[*chaintest.InMemoryStore]{
				To:    addr,
				Value: 1,
			},
			View: func() *chaintest.InMemoryStore {
				state := chaintest.NewInMemoryStore()

				require.NoError(t, storage.SetBalance(context.Background(), state, codec.EmptyAddress, 1))
				return state
			}(),
			Assertion: func(ctx context.Context, t *testing.T, state *chaintest.InMemoryStore) {
				receiverBalance, err := storage.GetBalance(ctx, state, addr)
				require.NoError(t, err)
				require.Equal(t, receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, state, codec.EmptyAddress)
				require.NoError(t, err)
				require.Equal(t, senderBalance, uint64(0))
			},
			ExpectedOutputs: &TransferResult{
				SenderBalance:   0,
				ReceiverBalance: 1,
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

	transferActionTest := &chaintest.ActionBenchmark[*chaintest.InMemoryStore]{
		Name:  "SimpleTransferBenchmark",
		Actor: from,
		Action: &Transfer[*chaintest.InMemoryStore]{
			To:    to,
			Value: 1,
		},
		NewViewF: func() *chaintest.InMemoryStore {
			store := chaintest.NewInMemoryStore()
			err := storage.SetBalance(context.Background(), store, from, 1)
			require.NoError(err)
			return store
		},
		Assertion: func(ctx context.Context, b *testing.B, view *chaintest.InMemoryStore) {
			toBalance, err := storage.GetBalance(ctx, view, to)
			require.NoError(err)
			require.Equal(uint64(1), toBalance)

			fromBalance, err := storage.GetBalance(ctx, view, from)
			require.NoError(err)
			require.Equal(uint64(0), fromBalance)
		},
	}

	ctx := context.Background()
	transferActionTest.Run(ctx, b)
}
