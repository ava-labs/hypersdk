// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fdsmr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/dsmr"
	"github.com/ava-labs/hypersdk/x/dsmr/dsmrtest"
)

var (
	_ dsmr.Interface[dsmrtest.Tx] = (*testDSMR)(nil)
	_ Bonder[dsmrtest.Tx]         = (*testBonder)(nil)
)

// Tests that txs that cannot be bonded are filtered out from calls to the
// underlying DSMR implementation
func TestNode_BuildChunk(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		bonder  testBonder
		txs     []dsmrtest.Tx
		wantTxs []dsmrtest.Tx
	}{
		{
			name:   "nil txs",
			bonder: testBonder{},
		},
		{
			name:    "empty txs",
			bonder:  testBonder{},
			txs:     []dsmrtest.Tx{},
			wantTxs: []dsmrtest.Tx{},
		},
		{
			name:   "single account - fails bond",
			bonder: testBonder{},
			txs: []dsmrtest.Tx{
				{
					ID:      ids.Empty,
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.EmptyAddress,
				},
			},
			wantTxs: []dsmrtest.Tx{},
		},
		{
			name: "single account - bonded",
			bonder: testBonder{
				pending: map[codec.Address]chan struct{}{
					codec.EmptyAddress: make(chan struct{}, 1),
				},
			},
			txs: []dsmrtest.Tx{
				{
					ID:      ids.Empty,
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.EmptyAddress,
				},
			},
			wantTxs: []dsmrtest.Tx{
				{
					ID:      ids.Empty,
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.EmptyAddress,
				},
			},
		},
		{
			name: "single account - some txs bonded",
			bonder: testBonder{
				pending: map[codec.Address]chan struct{}{
					codec.EmptyAddress: make(chan struct{}, 1),
				},
			},
			txs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.EmptyAddress,
				},
				{
					ID:      ids.ID{1},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.EmptyAddress,
				},
			},
			wantTxs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.EmptyAddress,
				},
			},
		},
		{
			name: "multiple accounts - all txs fail bond",
			bonder: testBonder{
				pending: map[codec.Address]chan struct{}{},
			},
			txs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{1},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{2},
				},
			},
			wantTxs: []dsmrtest.Tx{},
		},
		{
			name: "multiple accounts - all txs bonded",
			bonder: testBonder{
				pending: map[codec.Address]chan struct{}{
					{1}: make(chan struct{}, 1),
					{2}: make(chan struct{}, 1),
				},
			},
			txs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{1},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{2},
				},
			},
			wantTxs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{1},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{2},
				},
			},
		},
		{
			name: "multiple accounts - some txs bonded",
			bonder: testBonder{
				pending: map[codec.Address]chan struct{}{
					{1}: make(chan struct{}, 2),
					{2}: make(chan struct{}, 1),
				},
			},
			txs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{1},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{2},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{2},
				},
				{
					ID:      ids.ID{3},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{2},
				},
			},
			wantTxs: []dsmrtest.Tx{
				{
					ID:      ids.ID{0},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{1},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{1},
				},
				{
					ID:      ids.ID{2},
					Expiry:  now.Add(time.Hour).Unix(),
					Sponsor: codec.Address{2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			wantExpiry := int64(123)
			wantBeneficiary := codec.Address{1, 2, 3}
			wantErr := errors.New("foobar")
			n := New[testDSMR, dsmrtest.Tx](
				testDSMR{
					BuildChunkF: func(_ context.Context, gotTxs []dsmrtest.Tx, gotExpiry int64, gotBeneficiary codec.Address) error {
						r.Equal(tt.wantTxs, gotTxs)
						r.Equal(wantExpiry, gotExpiry)
						r.Equal(wantBeneficiary, gotBeneficiary)

						return wantErr
					},
				},
				tt.bonder,
			)
			r.ErrorIs(n.BuildChunk(
				context.Background(),
				tt.txs,
				wantExpiry,
				wantBeneficiary,
			), wantErr)
		})
	}
}

// Tests that txs are un-bonded when they are accepted
func TestUnbondOnAccept(t *testing.T) {
	r := require.New(t)

	b := testBonder{
		pending: map[codec.Address]chan struct{}{
			codec.EmptyAddress: make(chan struct{}, 1),
		},
	}
	expiry := time.Now().Add(time.Hour).Unix()
	txs := []dsmrtest.Tx{
		{
			ID:      ids.GenerateTestID(),
			Expiry:  expiry,
			Sponsor: codec.EmptyAddress,
		},
	}

	n := New[testDSMR, dsmrtest.Tx](
		testDSMR{
			AcceptF: func(_ context.Context, _ dsmr.Block) (dsmr.ExecutedBlock[dsmrtest.Tx], error) {
				return dsmr.ExecutedBlock[dsmrtest.Tx]{
					BlockHeader: dsmr.BlockHeader{},
					ID:          ids.ID{},
					Chunks: []dsmr.Chunk[dsmrtest.Tx]{
						{
							UnsignedChunk: dsmr.UnsignedChunk[dsmrtest.Tx]{
								Producer:    ids.NodeID{},
								Beneficiary: codec.Address{},
								Expiry:      0,
								Txs:         txs,
							},
							Signer:    [48]byte{},
							Signature: [96]byte{},
						},
					},
				}, nil
			},
		},
		b,
	)

	r.NoError(n.BuildChunk(
		context.Background(),
		txs,
		expiry,
		codec.EmptyAddress,
	))
	r.Len(b.pending[codec.EmptyAddress], 1)

	_, err := n.Accept(context.Background(), dsmr.Block{})
	r.NoError(err)
	r.Empty(b.pending[codec.EmptyAddress])
}

// Tests that txs are un-bonded after expiry
func TestUnbondOnExpiry(t *testing.T) {
	r := require.New(t)

	b := testBonder{
		pending: map[codec.Address]chan struct{}{
			codec.EmptyAddress: make(chan struct{}, 1),
		},
	}

	expiry := time.Now().Add(time.Second).Unix()
	txs := []dsmrtest.Tx{
		{
			ID:      ids.GenerateTestID(),
			Expiry:  expiry,
			Sponsor: codec.EmptyAddress,
		},
	}

	n := New[testDSMR, dsmrtest.Tx](testDSMR{}, b)
	r.NoError(n.BuildChunk(
		context.Background(),
		txs,
		expiry,
		codec.EmptyAddress,
	))
	// block until the tx is un-bonded
	<-b.pending[codec.EmptyAddress]
}

type testDSMR struct {
	BuildChunkF func(
		ctx context.Context,
		txs []dsmrtest.Tx,
		expiry int64,
		beneficiary codec.Address,
	) error
	AcceptF func(
		ctx context.Context,
		block dsmr.Block,
	) (dsmr.ExecutedBlock[dsmrtest.Tx], error)
}

func (t testDSMR) BuildChunk(ctx context.Context, txs []dsmrtest.Tx, expiry int64, beneficiary codec.Address) error {
	if t.BuildChunkF == nil {
		return nil
	}

	return t.BuildChunkF(ctx, txs, expiry, beneficiary)
}

func (testDSMR) BuildBlock(dsmr.Block, int64) (dsmr.Block, error) {
	panic("implement me")
}

func (testDSMR) Verify(context.Context, dsmr.Block, dsmr.Block) error {
	panic("implement me")
}

func (t testDSMR) Accept(ctx context.Context, block dsmr.Block) (dsmr.ExecutedBlock[dsmrtest.Tx], error) {
	if t.AcceptF == nil {
		return dsmr.ExecutedBlock[dsmrtest.Tx]{}, nil
	}

	return t.AcceptF(ctx, block)
}

type testBonder struct {
	pending map[codec.Address]chan struct{}
}

func (b testBonder) Bond(tx dsmrtest.Tx) bool {
	pending, ok := b.pending[tx.GetSponsor()]
	if !ok {
		return false
	}

	if len(pending) == cap(pending) {
		return false
	}

	pending <- struct{}{}
	return true
}

func (b testBonder) Unbond(tx dsmrtest.Tx) {
	<-b.pending[tx.GetSponsor()]
}
