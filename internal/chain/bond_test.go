// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth/authtest"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestBond(t *testing.T) {
	tests := []struct {
		name       string
		maxBalance *uint64
		tx         *chain.Transaction
		feeRate    uint64
		wantOk     bool
	}{
		{
			name: "no balance",
			tx: func() *chain.Transaction {
				tx, err := chain.NewTxData(
					&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				).Sign(&authtest.NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: 1,
		},
		{
			name:       "zero balance",
			maxBalance: newBalance(0),
			tx: func() *chain.Transaction {
				tx, err := chain.NewTxData(
					&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				).Sign(&authtest.NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: 1,
		},
		{
			name:       "balance less than fee",
			maxBalance: newBalance(1),
			tx: func() *chain.Transaction {
				tx, err := chain.NewTxData(
					&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				).Sign(&authtest.NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: 1,
		},
		{
			name:       "balance greater than or equal to fee",
			maxBalance: newBalance(1_000_000),
			tx: func() *chain.Transaction {
				tx, err := chain.NewTxData(
					&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				).Sign(&authtest.NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: 1,
			wantOk:  true,
		},
		{
			name:       "balance greater than or equal to fee - zero fee and zero balance",
			maxBalance: newBalance(0),
			tx: func() *chain.Transaction {
				tx, err := chain.NewTxData(
					&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				).Sign(&authtest.NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: 0,
			wantOk:  true,
		},
		{
			name:       "balance greater than or equal to fee - zero fee",
			maxBalance: newBalance(123),
			tx: func() *chain.Transaction {
				tx, err := chain.NewTxData(
					&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				).Sign(&authtest.NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			b := NewBonder(memdb.New())
			ts := tstate.New(0)
			view := ts.NewView(state.CompletePermissions, newDB(t), 0)

			if tt.maxBalance != nil {
				r.NoError(b.SetMaxBalance(context.Background(), view, codec.EmptyAddress, *tt.maxBalance))
			}

			ok, err := b.Bond(context.Background(), view, tt.tx, tt.feeRate)
			r.NoError(err)
			r.Equal(tt.wantOk, ok)
		})
	}
}

func TestUnbond_NoBondCalled(t *testing.T) {
	r := require.New(t)

	b := NewBonder(memdb.New())

	tx, err := chain.NewTxData(
		&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
		nil,
	).Sign(&authtest.NoAuthFactory{})
	r.NoError(err)

	r.NoError(b.Unbond(tx))
}

func TestUnbond_DuplicateUnbond(t *testing.T) {
	r := require.New(t)

	b := NewBonder(memdb.New())
	ts := tstate.New(0)
	view := ts.NewView(state.CompletePermissions, newDB(t), 0)

	tx, err := chain.NewTxData(
		&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
		nil,
	).Sign(&authtest.NoAuthFactory{})
	r.NoError(err)

	ok, err := b.Bond(context.Background(), view, tx, 0)
	r.True(ok)
	r.NoError(err)

	r.NoError(b.Unbond(tx))
	r.NoError(b.Unbond(tx))
}

func TestUnbond_UnbondAfterFailedBond(t *testing.T) {
	r := require.New(t)

	b := NewBonder(memdb.New())
	ts := tstate.New(0)
	view := ts.NewView(state.CompletePermissions, newDB(t), 0)

	tx, err := chain.NewTxData(
		&chain.Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
		nil,
	).Sign(&authtest.NoAuthFactory{})
	r.NoError(err)

	ok, err := b.Bond(context.Background(), view, tx, 1)
	r.False(ok)
	r.NoError(err)

	r.NoError(b.Unbond(tx))
}

func TestSetMaxBalanceDuringBond(t *testing.T) {
	r := require.New(t)
	b := NewBonder(memdb.New())

	ts := tstate.New(0)
	view := ts.NewView(state.CompletePermissions, newDB(t), 0)
	authFactory := &authtest.NoAuthFactory{}

	tx1, err := chain.NewTxData(&chain.Base{Timestamp: 0}, nil).Sign(authFactory)
	r.NoError(err)
	tx2, err := chain.NewTxData(&chain.Base{Timestamp: 1}, nil).Sign(authFactory)
	r.NoError(err)
	tx3, err := chain.NewTxData(&chain.Base{Timestamp: 2}, nil).Sign(authFactory)
	r.NoError(err)

	r.NoError(b.SetMaxBalance(
		context.Background(),
		view,
		codec.EmptyAddress,
		uint64(tx1.Size()+tx2.Size()+tx3.Size()),
	))

	ok, err := b.Bond(context.Background(), view, tx1, 1)
	r.NoError(err)
	r.True(ok)

	ok, err = b.Bond(context.Background(), view, tx2, 1)
	r.NoError(err)
	r.True(ok)

	r.NoError(b.SetMaxBalance(context.Background(), view, codec.EmptyAddress, 0))

	ok, err = b.Bond(context.Background(), view, tx3, 1)
	r.NoError(err)
	r.False(ok)

	r.NoError(b.Unbond(tx1))
	r.NoError(b.Unbond(tx2))
}

func newDB(t *testing.T) merkledb.MerkleDB {
	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{BranchFactor: 2},
	)
	require.NoError(t, err)
	return db
}

func newBalance(balance uint64) *uint64 {
	i := new(uint64)
	*i = balance

	return i
}
