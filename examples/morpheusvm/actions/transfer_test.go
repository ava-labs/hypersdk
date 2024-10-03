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
	"github.com/ava-labs/hypersdk/state/layout"
)

func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()
	stateLayout := layout.Default()

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
				_, err := storage.AddBalance(
					context.Background(),
					stateLayout,
					s,
					codec.EmptyAddress,
					0,
					true,
				)
				require.NoError(t, err)
				return s
			}(),
			ExpectedErr: storage.ErrInvalidBalance,
			StateLayout: stateLayout,
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
				require.NoError(t, storage.SetBalance(context.Background(), stateLayout, store, codec.EmptyAddress, 1))
				return store
			}(),
			StateLayout: stateLayout,
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				balance, err := storage.GetBalance(ctx, stateLayout, store, codec.EmptyAddress)
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
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: math.MaxUint64,
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), stateLayout, store, codec.EmptyAddress, 1))
				return store
			}(),
			StateLayout: stateLayout,
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
				store := chaintest.NewInMemoryStore()
				require.NoError(t, storage.SetBalance(context.Background(), stateLayout, store, codec.EmptyAddress, 1))
				return store
			}(),
			StateLayout: stateLayout,
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				receiverBalance, err := storage.GetBalance(ctx, stateLayout, store, addr)
				require.NoError(t, err)
				require.Equal(t, receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, stateLayout, store, codec.EmptyAddress)
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
	setupRequire := require.New(b)
	to := codec.CreateAddress(0, ids.GenerateTestID())
	from := codec.CreateAddress(0, ids.GenerateTestID())
	stateLayout := layout.Default()

	transferActionTest := &chaintest.ActionBenchmark{
		Name:  "SimpleTransferBenchmark",
		Actor: from,
		Action: &Transfer{
			To:    to,
			Value: 1,
		},
		ExpectedOutput: &TransferResult{
			SenderBalance:   0,
			ReceiverBalance: 1,
		},
		CreateState: func() state.Mutable {
			store := chaintest.NewInMemoryStore()
			err := storage.SetBalance(context.Background(), stateLayout, store, from, 1)
			setupRequire.NoError(err)
			return store
		},
		StateLayout: stateLayout,
		Assertion: func(ctx context.Context, b *testing.B, store state.Mutable) {
			require := require.New(b)
			toBalance, err := storage.GetBalance(ctx, stateLayout, store, to)
			require.NoError(err)
			require.Equal(uint64(1), toBalance)

			fromBalance, err := storage.GetBalance(ctx, stateLayout, store, from)
			require.NoError(err)
			require.Equal(uint64(0), fromBalance)
		},
	}

	ctx := context.Background()
	transferActionTest.Run(ctx, b)
}
