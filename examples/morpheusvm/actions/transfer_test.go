// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
)

func TestTransferAction(t *testing.T) {
	addr := codectest.NewRandomAddress()

	tests := []chaintest.ActionTest[struct{}]{
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
			Runtime: chain.Runtime[struct{}]{
				T:     struct{}{},
				State: chaintest.NewInMemoryStore(),
			},
			ExpectedErr: storage.ErrInvalidAddress,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Runtime: func() chain.Runtime[struct{}] {
				runtime := chain.Runtime[struct{}]{
					T:     struct{}{},
					State: chaintest.NewInMemoryStore(),
				}

				_, err := storage.AddBalance(
					context.Background(),
					runtime.State,
					codec.EmptyAddress,
					0,
					true,
				)
				require.NoError(t, err)
				return runtime
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
			Runtime: func() chain.Runtime[struct{}] {
				runtime := chain.Runtime[struct{}]{
					T:     struct{}{},
					State: chaintest.NewInMemoryStore(),
				}

				require.NoError(t, storage.SetBalance(context.Background(), runtime.State, codec.EmptyAddress, 1))
				return runtime
			}(),
			Assertion: func(ctx context.Context, t *testing.T, runtime chain.Runtime[struct{}]) {
				balance, err := storage.GetBalance(ctx, runtime.State, codec.EmptyAddress)
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
			Runtime: func() chain.Runtime[struct{}] {
				runtime := chain.Runtime[struct{}]{
					T:     struct{}{},
					State: chaintest.NewInMemoryStore(),
				}

				require.NoError(t, storage.SetBalance(context.Background(), runtime.State, codec.EmptyAddress, 1))
				return runtime
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
			Runtime: func() chain.Runtime[struct{}] {
				runtime := chain.Runtime[struct{}]{
					T:     struct{}{},
					State: chaintest.NewInMemoryStore(),
				}

				require.NoError(t, storage.SetBalance(context.Background(), runtime.State, codec.EmptyAddress, 1))
				return runtime
			}(),
			Assertion: func(ctx context.Context, t *testing.T, runtime chain.Runtime[struct{}]) {
				receiverBalance, err := storage.GetBalance(ctx, runtime.State, addr)
				require.NoError(t, err)
				require.Equal(t, receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, runtime.State, codec.EmptyAddress)
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

	transferActionTest := &chaintest.ActionBenchmark[struct{}]{
		Name:  "SimpleTransferBenchmark",
		Actor: from,
		Action: &Transfer{
			To:    to,
			Value: 1,
		},
		NewRuntimeF: func() chain.Runtime[struct{}] {
			store := chaintest.NewInMemoryStore()
			err := storage.SetBalance(context.Background(), store, from, 1)
			require.NoError(err)
			return chain.Runtime[struct{}]{
				T:     struct{}{},
				State: store,
			}
		},
		Assertion: func(ctx context.Context, b *testing.B, runtime chain.Runtime[struct{}]) {
			toBalance, err := storage.GetBalance(ctx, runtime.State, to)
			require.NoError(err)
			require.Equal(uint64(1), toBalance)

			fromBalance, err := storage.GetBalance(ctx, runtime.State, from)
			require.NoError(err)
			require.Equal(uint64(0), fromBalance)
		},
	}

	ctx := context.Background()
	transferActionTest.Run(ctx, b)
}
