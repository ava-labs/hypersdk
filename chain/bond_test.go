// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestSetMaxBalance(t *testing.T) {
	tests := []struct {
		name       string
		maxBalance uint32
	}{
		{
			name: "no balance",
		},
		{
			name:       "has balance",
			maxBalance: 123,
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
			mutable := state.NewSimpleMutable(db)
			address := codec.Address{1, 2, 3}
			r.NoError(b.SetMaxBalance(context.Background(), mutable, address, tt.maxBalance))

			for i := 0; i < int(tt.maxBalance); i++ {
				ok, err := b.Bond(
					context.Background(),
					mutable,
					&Transaction{Auth: TestAuth{SponsorF: address}},
				)
				r.NoError(err)
				r.True(ok)
			}

			ok, err := b.Bond(
				context.Background(),
				mutable,
				&Transaction{Auth: TestAuth{SponsorF: address}},
			)
			r.NoError(err)
			r.False(ok)
		})
	}
}

func TestBond(t *testing.T) {
	tests := []struct {
		name       string
		wantBond   []bool
		wantUnbond []error
		max        uint32
	}{
		{
			name: "bond with no balance",
			wantBond: []bool{
				false,
			},
		},
		{
			name: "bond less than balance",
			wantBond: []bool{
				true,
			},
			wantUnbond: []error{
				nil,
			},
			max: 2,
		},
		{
			name: "bond equal to balance",
			wantBond: []bool{
				true,
				true,
			},
			wantUnbond: []error{
				nil,
				nil,
			},
			max: 2,
		},
		{
			name: "bond more than balance",
			wantBond: []bool{
				true,
				true,
				false,
			},
			wantUnbond: []error{
				nil,
				nil,
			},
			max: 2,
		},
		{
			name: "unexpected unbond - no bond called",
			wantUnbond: []error{
				ErrMissingBond,
			},
			max: 2,
		},
		{
			name: "unexpected unbond - duplicate unbond called",
			wantBond: []bool{
				true,
			},
			wantUnbond: []error{
				nil,
				ErrMissingBond,
			},
			max: 2,
		},
		{
			name: "unexpected unbond - unbond called after unsuccessful bond",
			wantBond: []bool{
				true,
				false,
			},
			wantUnbond: []error{
				nil,
				ErrMissingBond,
			},
			max: 1,
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
			mutable := state.NewSimpleMutable(db)
			address := codec.Address{1, 2, 3}
			r.NoError(b.SetMaxBalance(
				context.Background(),
				mutable,
				address,
				tt.max,
			))

			for _, wantOk := range tt.wantBond {
				ok, err := b.Bond(
					context.Background(),
					mutable,
					&Transaction{Auth: TestAuth{SponsorF: address}},
				)
				r.NoError(err)
				r.Equal(wantOk, ok)
			}

			for _, wantErr := range tt.wantUnbond {
				r.ErrorIs(
					b.Unbond(
						context.Background(),
						mutable,
						&Transaction{Auth: TestAuth{SponsorF: address}},
					),
					wantErr,
				)
			}
		})
	}
}

func TestSetMaxBalanceDuringBond(t *testing.T) {
	r := require.New(t)
	b := Bonder{}

	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{BranchFactor: 2},
	)
	r.NoError(err)
	mutable := state.NewSimpleMutable(db)
	address := codec.Address{1, 2, 3}
	r.NoError(b.SetMaxBalance(context.Background(), mutable, address, 3))

	tx1 := &Transaction{
		Auth: TestAuth{
			SponsorF: address,
		},
	}

	tx2 := &Transaction{
		Auth: TestAuth{
			SponsorF: address,
		},
	}

	tx3 := &Transaction{
		Auth: TestAuth{
			SponsorF: address,
		},
	}

	ok, err := b.Bond(context.Background(), mutable, tx1)
	r.NoError(err)
	r.True(ok)

	ok, err = b.Bond(context.Background(), mutable, tx2)
	r.NoError(err)
	r.True(ok)

	r.NoError(b.SetMaxBalance(context.Background(), mutable, address, 0))

	ok, err = b.Bond(context.Background(), mutable, tx3)
	r.NoError(err)
	r.False(ok)

	r.NoError(b.Unbond(context.Background(), mutable, tx1))
	r.NoError(b.Unbond(context.Background(), mutable, tx2))
}

type TestAuth struct {
	TypeID        uint8
	Start         int64
	End           int64
	ComputeUnitsF uint64
	VerifyErr     error
	ActorF        codec.Address
	SponsorF      codec.Address
}

func (t TestAuth) GetTypeID() uint8 {
	return t.TypeID
}

func (t TestAuth) ValidRange(Rules) (start int64, end int64) {
	return t.Start, t.End
}

func (TestAuth) Marshal(*codec.Packer) {}

func (TestAuth) Size() int { return 0 }

func (t TestAuth) ComputeUnits(Rules) uint64 {
	return t.ComputeUnitsF
}

func (t TestAuth) Verify(context.Context, []byte) error {
	return t.VerifyErr
}

func (t TestAuth) Actor() codec.Address {
	return t.ActorF
}

func (t TestAuth) Sponsor() codec.Address {
	return t.SponsorF
}
