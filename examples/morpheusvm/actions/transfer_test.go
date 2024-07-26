// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/hex"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

func TestTransferAction(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)
	emptyBalanceKey := storage.BalanceKey(codec.EmptyAddress)
	oneAddr := createAddressWithByte(t, 1)

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
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				require := require.New(t)
				balance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
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
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				require := require.New(t)
				receiverBalance, err := storage.GetBalance(ctx, store, oneAddr)
				require.NoError(err)
				require.Equal(receiverBalance, uint64(1))
				senderBalance, err := storage.GetBalance(ctx, store, codec.EmptyAddress)
				require.NoError(err)
				require.Equal(senderBalance, uint64(0))
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

func createAddressWithByte(t *testing.T, b byte) codec.Address {
	addrSlice := make([]byte, codec.AddressLen)
	for i := range addrSlice {
		addrSlice[i] = b
	}
	addr, err := codec.ToAddress(addrSlice)
	require.NoError(t, err)
	return addr
}
func TestTransferMarshalSpec(t *testing.T) {
	tests := []struct {
		name     string
		transfer Transfer
		expected string
	}{
		{
			name: "Zero value",
			transfer: Transfer{
				To:    createAddressWithByte(t, 7),
				Value: 0,
				Memo:  []byte("test memo"),
			},
			expected: "07070707070707070707070707070707070707070707070707070707070707070700000000000000000000000974657374206d656d6f",
		},
		{
			name: "Max uint64 value",
			transfer: Transfer{
				To:    createAddressWithByte(t, 7),
				Value: math.MaxUint64,
				Memo:  []byte("another memo"),
			},
			expected: "070707070707070707070707070707070707070707070707070707070707070707ffffffffffffffff0000000c616e6f74686572206d656d6f",
		},
		{
			name: "Empty address",
			transfer: Transfer{
				To:    codec.EmptyAddress,
				Value: 123,
				Memo:  []byte("memo"),
			},
			expected: "000000000000000000000000000000000000000000000000000000000000000000000000000000007b000000046d656d6f",
		},
		{
			name: "Empty memo",
			transfer: Transfer{
				To:    createAddressWithByte(t, 5),
				Value: 456,
				Memo:  []byte{},
			},
			expected: "05050505050505050505050505050505050505050505050505050505050505050500000000000001c800000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := codec.NewWriter(0, consts.NetworkSizeLimit)
			tt.transfer.Marshal(p)
			require.Equal(t, tt.expected, hex.EncodeToString(p.Bytes()))
		})
	}
}
