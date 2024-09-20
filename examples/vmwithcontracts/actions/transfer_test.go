// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestTransferAction(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)
	emptyBalanceKey := storage.BalanceKey(codec.EmptyAddress)
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
			Name:  "InvalidStateKey",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Runtime: func() chain.Runtime[struct{}] {
				return chain.Runtime[struct{}]{
					T:     struct{}{},
					State: ts.NewView(make(state.Keys), map[string][]byte{}),
				}
			}(),
			ExpectedErr: tstate.ErrInvalidKeyOrPermission,
		},
		{
			Name:  "NotEnoughBalance",
			Actor: codec.EmptyAddress,
			Action: &Transfer{
				To:    codec.EmptyAddress,
				Value: 1,
			},
			Runtime: func() chain.Runtime[struct{}] {
				keys := make(state.Keys)
				keys.Add(string(emptyBalanceKey), state.Read)
				tsv := ts.NewView(keys, map[string][]byte{})
				return chain.Runtime[struct{}]{
					T:     struct{}{},
					State: tsv,
				}
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
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				return chain.Runtime[struct{}]{
					T:     struct{}{},
					State: ts.NewView(keys, store.Storage),
				}
			}(),
			Assertion: func(ctx context.Context, t *testing.T, runtime chain.Runtime[struct{}]) {
				require := require.New(t)
				balance, err := storage.GetBalance(ctx, runtime.State, codec.EmptyAddress)
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
			Runtime: func() chain.Runtime[struct{}] {
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				return chain.Runtime[struct{}]{
					T:     struct{}{},
					State: ts.NewView(keys, store.Storage),
				}
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
				keys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				req.NoError(storage.SetBalance(context.Background(), store, codec.EmptyAddress, 1))
				keys.Add(string(emptyBalanceKey), state.All)
				keys.Add(string(storage.BalanceKey(addr)), state.All)

				return chain.Runtime[struct{}]{
					T:     struct{}{},
					State: ts.NewView(keys, store.Storage),
				}
			}(),
			Assertion: func(ctx context.Context, t *testing.T, runtime chain.Runtime[struct{}]) {
				require := require.New(t)
				receiverBalance, err := storage.GetBalance(ctx, runtime.State, addr)
				require.NoError(err)
				require.Equal(receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, runtime.State, codec.EmptyAddress)
				require.NoError(err)
				require.Equal(senderBalance, uint64(0))
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
