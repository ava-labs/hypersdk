// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/stretchr/testify/require"
)

func TestBond(t *testing.T) {
	tests := []struct {
		name       string
		maxBalance *big.Int
		tx         *Transaction
		feeRate    *big.Int
		wantOk     bool
	}{
		{
			name: "no balance",
			tx: func() *Transaction {
				txData := NewTxData(
					&Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				)

				tx, err := txData.Sign(&NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: big.NewInt(1),
		},
		{
			name:       "zero balance",
			maxBalance: big.NewInt(0),
			tx: func() *Transaction {
				txData := NewTxData(
					&Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				)

				tx, err := txData.Sign(&NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: big.NewInt(1),
		},
		{
			name:       "balance less than fee",
			maxBalance: big.NewInt(1),
			tx: func() *Transaction {
				txData := NewTxData(
					&Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				)

				tx, err := txData.Sign(&NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: big.NewInt(1),
		},
		{
			name:       "balance greater than or equal to fee",
			maxBalance: big.NewInt(1_000_000),
			tx: func() *Transaction {
				txData := NewTxData(
					&Base{Timestamp: 123, ChainID: ids.Empty, MaxFee: 456},
					nil,
				)

				tx, err := txData.Sign(&NoAuthFactory{})
				require.NoError(t, err)

				return tx
			}(),
			feeRate: big.NewInt(1),
			wantOk:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			b := Bonder{}
			db, err := merkledb.New(
				context.Background(),
				memdb.New(),
				merkledb.Config{BranchFactor: 2},
			)
			r.NoError(err)
			ts := tstate.New(0)
			view := ts.NewView(state.CompletePermissions, db, 0)

			if tt.maxBalance != nil {
				r.NoError(b.SetMaxBalance(context.Background(), view, codec.EmptyAddress, tt.maxBalance))
			}

			ok, err := b.Bond(context.Background(), view, tt.tx, tt.feeRate)
			r.NoError(err)
			r.Equal(tt.wantOk, ok)
		})
	}
}

//func TestUnbond(t *testing.T) {
//	tests := []struct {
//		name       string
//		wantBond   []bool
//		wantUnbond []error
//		max        *big.Int
//	}{
//		{
//			name: "bond with no balance",
//			wantBond: []bool{
//				false,
//			},
//		},
//		{
//			name: "bond less than balance",
//			wantBond: []bool{
//				true,
//			},
//			wantUnbond: []error{
//				nil,
//			},
//			max: 2,
//		},
//		{
//			name: "bond equal to balance",
//			wantBond: []bool{
//				true,
//				true,
//			},
//			wantUnbond: []error{
//				nil,
//				nil,
//			},
//			max: 2,
//		},
//		{
//			name: "bond more than balance",
//			wantBond: []bool{
//				true,
//				true,
//				false,
//			},
//			wantUnbond: []error{
//				nil,
//				nil,
//			},
//			max: 2,
//		},
//		{
//			name: "unexpected unbond - no bond called",
//			wantUnbond: []error{
//				ErrMissingBond,
//			},
//			max: 2,
//		},
//		{
//			name: "unexpected unbond - duplicate unbond called",
//			wantBond: []bool{
//				true,
//			},
//			wantUnbond: []error{
//				nil,
//				ErrMissingBond,
//			},
//			max: 2,
//		},
//		{
//			name: "unexpected unbond - unbond called after unsuccessful bond",
//			wantBond: []bool{
//				true,
//				false,
//			},
//			wantUnbond: []error{
//				nil,
//				ErrMissingBond,
//			},
//			max: 1,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			r := require.New(t)
//			b := Bonder{}
//
//			db, err := merkledb.New(
//				context.Background(),
//				memdb.New(),
//				merkledb.Config{BranchFactor: 2},
//			)
//			r.NoError(err)
//			mutable := state.NewSimpleMutable(db)
//			address := codec.Address{1, 2, 3}
//			r.NoError(b.SetMaxBalance(
//				context.Background(),
//				mutable,
//				address,
//				tt.max,
//			))
//
//			for _, wantOk := range tt.wantBond {
//				ok, err := b.Bond(
//					context.Background(),
//					mutable,
//					&Transaction{Auth: NoAuth{SponsorF: address}},
//				)
//				r.NoError(err)
//				r.Equal(wantOk, ok)
//			}
//
//			for _, wantErr := range tt.wantUnbond {
//				r.ErrorIs(
//					b.Unbond(
//						context.Background(),
//						mutable,
//						&Transaction{Auth: NoAuth{SponsorF: address}},
//					),
//					wantErr,
//				)
//			}
//		})
//	}
//}

func TestSetMaxBalanceDuringBond(t *testing.T) {
	r := require.New(t)
	b := Bonder{}

	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{BranchFactor: 2},
	)
	r.NoError(err)
	ts := tstate.New(0)
	view := ts.NewView(state.CompletePermissions, db, 0)
	authFactory := &NoAuthFactory{}

	tx1, err := NewTxData(&Base{Timestamp: 0}, nil).Sign(authFactory)
	r.NoError(err)
	tx2, err := NewTxData(&Base{Timestamp: 1}, nil).Sign(authFactory)
	r.NoError(err)
	tx3, err := NewTxData(&Base{Timestamp: 2}, nil).Sign(authFactory)
	r.NoError(err)

	r.NoError(b.SetMaxBalance(
		context.Background(),
		view,
		codec.EmptyAddress,
		big.NewInt(int64(tx1.Size()+tx2.Size()+tx3.Size())),
	))

	feeRate := big.NewInt(1)
	ok, err := b.Bond(context.Background(), view, tx1, feeRate)
	r.NoError(err)
	r.True(ok)

	ok, err = b.Bond(context.Background(), view, tx2, feeRate)
	r.NoError(err)
	r.True(ok)

	r.NoError(b.SetMaxBalance(context.Background(), view, codec.EmptyAddress, big.NewInt(0)))

	ok, err = b.Bond(context.Background(), view, tx3, feeRate)
	r.NoError(err)
	r.False(ok)

	r.NoError(b.Unbond(context.Background(), view, tx1))
	r.NoError(b.Unbond(context.Background(), view, tx2))
}

type NoAuthFactory struct{}

func (NoAuthFactory) Sign([]byte) (Auth, error) {
	return NoAuth{}, nil
}

func (NoAuthFactory) MaxUnits() (uint64, uint64) {
	return 0, 0
}

func (n NoAuthFactory) Address() codec.Address {
	return codec.EmptyAddress
}

type NoAuth struct{}

func (t NoAuth) GetTypeID() uint8 {
	return 0
}

func (t NoAuth) ValidRange(Rules) (start int64, end int64) {
	return 0, math.MaxInt64
}

func (NoAuth) Marshal(*codec.Packer) {}

func (NoAuth) Size() int { return 0 }

func (NoAuth) ComputeUnits(Rules) uint64 {
	return 0
}

func (NoAuth) Verify(context.Context, []byte) error {
	return nil
}

func (NoAuth) Actor() codec.Address {
	return codec.EmptyAddress
}

func (n NoAuth) Sponsor() codec.Address {
	return codec.EmptyAddress
}
