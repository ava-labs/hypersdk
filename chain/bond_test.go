// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

func TestSetMaxBalance(t *testing.T) {
	tests := []struct {
		name       string
		prev       uint32
		maxBalance uint32
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			b := NewBonder(memdb.New())

			address := codec.Address{1, 2, 3}
			prev, err := b.SetMaxBalance(address, 1)
			r.NoError(err)
			r.Zero(prev)

			prev, err = b.SetMaxBalance(address, 2)
			r.NoError(err)
			r.Equal(1, prev)

			for i := 0; i < 2; i++ {
				ok, err := b.Bond(&Transaction{
					Auth: TestAuth{
						SponsorF: address,
					},
				})
				r.NoError(err)
				r.True(ok)
			}
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
			b := NewBonder(memdb.New())

			address := codec.Address{1, 2, 3}
			_, err := b.SetMaxBalance(address, tt.max)
			r.NoError(err)

			for _, wantOk := range tt.wantBond {
				ok, err := b.Bond(&Transaction{
					Auth: TestAuth{
						SponsorF: address,
					},
				})
				r.NoError(err)
				r.Equal(wantOk, ok)
			}

			for _, wantErr := range tt.wantUnbond {
				r.ErrorIs(
					b.Unbond(&Transaction{Auth: TestAuth{SponsorF: address}}),
					wantErr,
				)
			}
		})
	}
}

type TestAuth struct {
	SponsorF codec.Address
}

func (TestAuth) GetTypeID() uint8 {
	return 0
}

func (TestAuth) ValidRange(Rules) (start int64, end int64) {
	return 0, 0
}

func (TestAuth) Marshal(p *codec.Packer) {}

func (TestAuth) Size() int {
	return 0
}

func (TestAuth) ComputeUnits(Rules) uint64 {
	return 0
}

func (TestAuth) Verify(context.Context, []byte) error {
	return nil
}

func (TestAuth) Actor() codec.Address {
	return codec.EmptyAddress
}

func (t TestAuth) Sponsor() codec.Address {
	return t.SponsorF
}
