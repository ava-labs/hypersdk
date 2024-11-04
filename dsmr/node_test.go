// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ Tx                                                                         = (*tx)(nil)
	_ Marshaler[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse] = (*getChunkSignatureMarshaler)(nil)
	_ Verifier[tx]                                                               = (*failVerifier)(nil)
)

// Test that chunks can be built through Node.NewChunk
func TestNode_NewChunk(t *testing.T) {
	tests := []struct {
		name    string
		txs     []tx
		expiry  time.Time
		wantErr error
	}{
		{
			name:    "empty chunk",
			expiry:  time.Now(),
			wantErr: ErrEmptyChunk,
		},
		{
			name: "chunk with 1 tx",
			txs: []tx{
				{
					ID:     ids.GenerateTestID(),
					Expiry: 1,
				},
			},
			expiry: time.Now(),
		},
		{
			name: "chunk with multiple txs",
			txs: []tx{
				{
					ID:     ids.GenerateTestID(),
					Expiry: 1,
				},
				{
					ID:     ids.GenerateTestID(),
					Expiry: 2,
				},
				{
					ID:     ids.GenerateTestID(),
					Expiry: 3,
				},
			},
			expiry: time.Now(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			sk, err := bls.NewSecretKey()
			r.NoError(err)

			nodeID := ids.GenerateTestNodeID()
			beneficiary := codec.CreateAddress(0x0, ids.GenerateTestID())
			node, err := New[tx](
				nodeID,
				sk,
				beneficiary,
				NoVerifier[tx]{},
				nil,
			)
			r.NoError(err)

			chunk, err := node.NewChunk(tt.txs, tt.expiry)
			r.ErrorIs(err, tt.wantErr)
			if err != nil {
				return
			}

			r.Equal(nodeID, chunk.Producer)
			r.Equal(beneficiary, chunk.Beneficiary)
			r.Equal(tt.expiry.Unix(), chunk.Expiry)
			r.ElementsMatch(tt.txs, chunk.Txs)
			// TODO verify signature + signer
			r.NotEmpty(chunk.Signature)
			r.NotEmpty(chunk.Signer)
		})
	}
}

// Tests that chunks referenced in accepted blocks are available over p2p
func TestNode_GetChunk_AvailableChunk(t *testing.T) {
	r := require.New(t)
	sk, err := bls.NewSecretKey()
	r.NoError(err)

	node, err := New[tx](
		ids.GenerateTestNodeID(),
		sk,
		codec.Address{},
		NoVerifier[tx]{},
		nil,
	)
	r.NoError(err)

	chunk, err := node.NewChunk(
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		time.Now(),
	)
	r.NoError(err)

	blk := node.NewBlock()
	r.NoError(node.Accept(context.Background(), blk))

	client := NewGetChunkClient(p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response *dsmr.GetChunkResponse, err error) {
		defer close(done)

		gotChunk, err := newChunkFromProto[tx](response.Chunk)
		r.NoError(err)
		r.Equal(chunk, gotChunk)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		set.Of(ids.EmptyNodeID),
		&dsmr.GetChunkRequest{
			ChunkId: chunk.id[:],
			Expiry:  chunk.Expiry,
		},
		onResponse,
	))
	<-done
}

// Tests that pending chunks are not available over p2p
func TestNode_GetChunk_PendingChunk(t *testing.T) {
	r := require.New(t)
	sk, err := bls.NewSecretKey()
	r.NoError(err)

	node, err := New[tx](
		ids.GenerateTestNodeID(),
		sk,
		codec.Address{},
		NoVerifier[tx]{},
		nil,
	)
	r.NoError(err)

	chunk, err := node.NewChunk(
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		time.Now(),
	)
	r.NoError(err)

	client := NewGetChunkClient(p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response *dsmr.GetChunkResponse, err error) {
		defer close(done)

		r.ErrorIs(err, ErrChunkNotAvailable)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		set.Of(ids.EmptyNodeID),
		&dsmr.GetChunkRequest{
			ChunkId: chunk.id[:],
			Expiry:  chunk.Expiry,
		},
		onResponse,
	))
	<-done
}

// Tests that unknown chunks are not available over p2p
func TestNode_GetChunk_UnknownChunk(t *testing.T) {
	r := require.New(t)
	sk, err := bls.NewSecretKey()
	r.NoError(err)

	node, err := New[tx](
		ids.GenerateTestNodeID(),
		sk,
		codec.Address{},
		NoVerifier[tx]{},
		nil,
	)
	r.NoError(err)

	client := NewGetChunkClient(p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response *dsmr.GetChunkResponse, err error) {
		defer close(done)

		r.ErrorIs(err, ErrChunkNotAvailable)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		set.Of(ids.EmptyNodeID),
		&dsmr.GetChunkRequest{
			ChunkId: ids.Empty[:],
			Expiry:  123,
		},
		onResponse,
	))
	<-done
}

// Tests that a Node serves chunks it built over GetChunk
func TestNode_BuiltChunksAvailableOverGetChunk(t *testing.T) {
	type availableChunk struct {
		txs    []tx
		expiry time.Time
	}

	tests := []struct {
		name            string
		availableChunks []availableChunk
	}{
		{
			name: "1 chunk with 1 tx",
			availableChunks: []availableChunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 0,
						},
					},
					expiry: time.Now(),
				},
			},
		},
		{
			name: "1 chunk with 3 txs",
			availableChunks: []availableChunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 0,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: time.Now(),
				},
			},
		},
		{
			name: "3 chunks with 1 tx",
			availableChunks: []availableChunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 0,
						},
					},
					expiry: time.Now(),
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry: time.Now(),
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: time.Now(),
				},
			},
		},
		{
			name: "3 chunks with 3 txs",
			availableChunks: []availableChunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 0,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: time.Now(),
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 3,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 4,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 5,
						},
					},
					expiry: time.Now(),
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 6,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 7,
						},
						{
							ID:     ids.GenerateTestID(),
							Expiry: 8,
						},
					},
					expiry: time.Now(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			nodeID := ids.GenerateTestNodeID()
			sk, err := bls.NewSecretKey()
			r.NoError(err)
			beneficiary := codec.CreateAddress(123, ids.GenerateTestID())

			node, err := New[tx](
				nodeID,
				sk,
				beneficiary,
				NoVerifier[tx]{},
				nil,
			)
			r.NoError(err)

			// Build some chunks
			wantChunks := make([]Chunk[tx], 0)
			for _, args := range tt.availableChunks {
				chunk, err := node.NewChunk(args.txs, args.expiry)
				r.NoError(err)

				wantChunks = append(wantChunks, chunk)
			}

			block := node.NewBlock()
			r.NoError(node.Accept(context.Background(), block))

			client := NewGetChunkClient(p2ptest.NewClient(
				t,
				context.Background(),
				node.GetChunkHandler,
				ids.EmptyNodeID,
				ids.EmptyNodeID,
			))

			wg := &sync.WaitGroup{}
			wg.Add(len(tt.availableChunks))

			// Node must serve GetChunk requests for chunks that it accepted
			gotChunks := make(map[ids.ID]Chunk[tx], 0)
			for _, chunk := range wantChunks {
				r.NoError(client.AppRequest(
					context.Background(),
					set.Of(ids.EmptyNodeID),
					&dsmr.GetChunkRequest{
						ChunkId: chunk.id[:],
						Expiry:  chunk.Expiry,
					},
					func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkResponse, err error) {
						defer wg.Done()

						r.NoError(err)

						gotChunk, err := newChunkFromProto[tx](response.Chunk)
						r.NoError(err)

						gotChunks[gotChunk.id] = gotChunk
					},
				))
			}

			wg.Wait()

			for _, chunk := range wantChunks {
				r.Contains(gotChunks, chunk.id)

				gotChunk := gotChunks[chunk.id]
				r.Equal(nodeID, gotChunk.Producer)
				r.Equal(chunk.Expiry, gotChunk.Expiry)
				r.Equal(beneficiary, gotChunk.Beneficiary)
				r.ElementsMatch(chunk.Txs, gotChunk.Txs)
				// TODO verify
				r.NotEmpty(chunk.Signer)
				r.NotEmpty(chunk.Signature)
			}
		})
	}
}

// Node should be willing to sign valid chunks
func TestNode_GetChunkSignature(t *testing.T) {
	tests := []struct {
		name     string
		verifier Verifier[tx]
		wantErr  error
	}{
		{
			name:     "invalid chunk",
			verifier: failVerifier{},
			wantErr:  ErrInvalidChunk,
		},
		{
			name:     "valid chunk",
			verifier: NoVerifier[tx]{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			sk, err := bls.NewSecretKey()
			r.NoError(err)
			pk := bls.PublicFromSecretKey(sk)

			node1, err := New[tx](
				ids.EmptyNodeID,
				sk,
				codec.Address{},
				tt.verifier,
				nil,
			)
			r.NoError(err)

			client := NewGetChunkSignatureClient(p2ptest.NewClient(
				t,
				context.Background(),
				node1.GetChunkSignatureHandler,
				ids.EmptyNodeID,
				ids.EmptyNodeID,
			))

			node2, err := New[tx](
				ids.EmptyNodeID,
				sk,
				codec.Address{},
				tt.verifier,
				nil,
			)
			r.NoError(err)
			chunk, err := node2.NewChunk([]tx{{ID: ids.Empty, Expiry: 123}}, time.Now())
			r.NoError(err)

			done := make(chan struct{})
			onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkSignatureResponse, err error) {
				defer close(done)

				r.ErrorIs(err, tt.wantErr)
				if err != nil {
					return
				}

				r.Equal(bls.PublicKeyToCompressedBytes(pk), response.Signer)
				signature, err := bls.SignatureFromBytes(response.Signature)
				r.NoError(err)

				r.True(bls.Verify(pk, signature, chunk.bytes))
			}

			protoChunk, err := newProtoChunk(chunk)
			r.NoError(err)

			r.NoError(client.AppRequest(
				context.Background(),
				set.Of(ids.EmptyNodeID),
				&dsmr.GetChunkSignatureRequest{Chunk: protoChunk},
				onResponse,
			))

			<-done
		})
	}
}

// Node should not sign duplicate chunks that have already been accepted
func TestNode_GetChunkSignature_DuplicateChunk(t *testing.T) {
	r := require.New(t)
	sk, err := bls.NewSecretKey()
	r.NoError(err)

	node, err := New[tx](
		ids.EmptyNodeID,
		sk,
		codec.Address{},
		NoVerifier[tx]{},
		nil,
	)
	r.NoError(err)

	client := NewGetChunkSignatureClient(p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkSignatureHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	// Accept a chunk
	r.NoError(err)
	chunk, err := node.NewChunk([]tx{{ID: ids.Empty, Expiry: 123}}, time.Now())
	r.NoError(err)
	blk := node.NewBlock()
	r.NoError(node.Accept(context.Background(), blk))

	done := make(chan struct{})
	onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkSignatureResponse, err error) {
		defer close(done)

		r.ErrorIs(err, ErrDuplicateChunk)
	}

	protoChunk, err := newProtoChunk(chunk)
	r.NoError(err)

	r.NoError(client.AppRequest(
		context.Background(),
		set.Of(ids.EmptyNodeID),
		&dsmr.GetChunkSignatureRequest{Chunk: protoChunk},
		onResponse,
	))

	<-done
}

type tx struct {
	ID     ids.ID `serialize:"true"`
	Expiry int64  `serialize:"true"`
}

func (t tx) GetID() ids.ID {
	return t.ID
}

func (t tx) GetExpiry() time.Time {
	return time.Unix(0, t.Expiry)
}

type failVerifier struct{}

func (f failVerifier) Verify(Chunk[tx]) error {
	return errors.New("fail")
}
