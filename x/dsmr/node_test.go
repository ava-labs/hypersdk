// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/snow/validators/validatorstest"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ Tx           = (*tx)(nil)
	_ Verifier[tx] = (*failVerifier)(nil)
)

type testingChainIndex struct {
	blocks map[ids.ID]validitywindow.ExecutionBlock[*ChunkCertificate]
}

func (ti *testingChainIndex) GetExecutionBlock(_ context.Context, id ids.ID) (validitywindow.ExecutionBlock[*ChunkCertificate], error) {
	if blk, has := ti.blocks[id]; has {
		return blk, nil
	}
	return nil, nil
}

func (ti *testingChainIndex) set(id ids.ID, blk validitywindow.ExecutionBlock[*ChunkCertificate]) {
	ti.blocks[id] = blk
}

func newTestingChainIndexer() *testingChainIndex {
	return &testingChainIndex{
		blocks: make(map[ids.ID]validitywindow.ExecutionBlock[*ChunkCertificate]),
	}
}

// Test that chunks can be built through Node.NewChunk
func TestNode_BuildChunk(t *testing.T) {
	tests := []struct {
		name        string
		txs         []tx
		expiry      int64
		beneficiary codec.Address
		wantErr     error
	}{
		{
			name:        "empty chunk",
			expiry:      123,
			beneficiary: codec.Address{123},
			wantErr:     ErrEmptyChunk,
		},
		{
			name: "chunk with 1 tx",
			txs: []tx{
				{
					ID:     ids.GenerateTestID(),
					Expiry: 1,
				},
			},
			expiry:      123,
			beneficiary: codec.Address{123},
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
			expiry:      123,
			beneficiary: codec.Address{123},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			networkID := uint32(123)
			chainID := ids.Empty
			sk, err := bls.NewSecretKey()
			r.NoError(err)
			pk := bls.PublicFromSecretKey(sk)
			signer := warp.NewSigner(sk, networkID, chainID)
			nodeID := ids.GenerateTestNodeID()
			node, err := New[tx](
				nodeID,
				networkID,
				chainID,
				pk,
				signer,
				NoVerifier[tx]{},
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				nil,
				logging.NoLog{},
				trace.Noop,
				newTestingChainIndexer(),
			)
			r.NoError(err)

			chunk, err := node.BuildChunk(
				context.Background(),
				tt.txs,
				tt.expiry,
				tt.beneficiary,
			)
			r.ErrorIs(err, tt.wantErr)
			if err != nil {
				return
			}

			r.Equal(nodeID, chunk.Producer)
			r.Equal(tt.beneficiary, chunk.Beneficiary)
			r.Equal(tt.expiry, chunk.Expiry)
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

	networkID := uint32(123)
	chainID := ids.Empty
	sk, err := bls.NewSecretKey()
	r.NoError(err)
	pk := bls.PublicFromSecretKey(sk)
	signer := warp.NewSigner(sk, networkID, chainID)
	node, err := New[tx](
		ids.GenerateTestNodeID(),
		networkID,
		chainID,
		pk,
		signer,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	chunk, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	blk, err := node.BuildBlock(
		context.Background(),
		Block{
			ParentID: ids.GenerateTestID(),
			Hght:     0,
			Tmstmp:   1,
		},
		2,
	)
	r.NoError(err)
	r.NoError(node.Accept(context.Background(), blk))

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
		defer close(done)
		r.NoError(err)

		r.NoError(err)
		r.Equal(chunk, response)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		ids.EmptyNodeID,
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

	networkID := uint32(123)
	chainID := ids.Empty
	sk, err := bls.NewSecretKey()
	r.NoError(err)
	pk := bls.PublicFromSecretKey(sk)
	signer := warp.NewSigner(sk, networkID, chainID)
	node, err := New[tx](
		ids.GenerateTestNodeID(),
		networkID,
		chainID,
		pk,
		signer,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	chunk, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ Chunk[tx], err error) {
		defer close(done)

		r.ErrorIs(err, ErrChunkNotAvailable)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		ids.EmptyNodeID,
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

	networkID := uint32(123)
	chainID := ids.Empty
	sk, err := bls.NewSecretKey()
	r.NoError(err)
	pk := bls.PublicFromSecretKey(sk)
	signer := warp.NewSigner(sk, networkID, chainID)
	node, err := New[tx](
		ids.GenerateTestNodeID(),
		networkID,
		chainID,
		pk,
		signer,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		node.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ Chunk[tx], err error) {
		defer close(done)

		r.ErrorIs(err, ErrChunkNotAvailable)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		ids.EmptyNodeID,
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
		txs         []tx
		expiry      int64
		beneficiary codec.Address
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
					expiry:      123,
					beneficiary: codec.Address{},
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
					expiry:      123,
					beneficiary: codec.Address{},
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
					expiry:      123,
					beneficiary: codec.Address{0},
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry:      123,
					beneficiary: codec.Address{1},
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry:      123,
					beneficiary: codec.Address{2},
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
					expiry: 123,
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
					expiry: 123,
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
					expiry: 123,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			nodeID := ids.GenerateTestNodeID()
			networkID := uint32(123)
			chainID := ids.Empty
			sk, err := bls.NewSecretKey()
			r.NoError(err)
			pk := bls.PublicFromSecretKey(sk)
			signer := warp.NewSigner(sk, networkID, chainID)
			node, err := New[tx](
				nodeID,
				networkID,
				chainID,
				pk,
				signer,
				NoVerifier[tx]{},
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				nil,
				logging.NoLog{},
				trace.Noop,
				newTestingChainIndexer(),
			)
			r.NoError(err)

			// Build some chunks
			wantChunks := make([]Chunk[tx], 0)
			for _, args := range tt.availableChunks {
				chunk, err := node.BuildChunk(
					context.Background(),
					args.txs,
					args.expiry,
					args.beneficiary,
				)
				r.NoError(err)

				wantChunks = append(wantChunks, chunk)
			}

			block, err := node.BuildBlock(
				context.Background(),
				Block{
					ParentID: ids.GenerateTestID(),
					Hght:     0,
					Tmstmp:   1,
				},
				2,
			)
			r.NoError(err)
			r.NoError(node.Accept(context.Background(), block))

			client := NewGetChunkClient[tx](p2ptest.NewClient(
				t,
				context.Background(),
				node.GetChunkHandler,
				ids.EmptyNodeID,
				ids.EmptyNodeID,
			))

			// Node must serve GetChunk requests for chunks that it accepted
			gotChunks := make(map[ids.ID]Chunk[tx], 0)
			for _, chunk := range wantChunks {
				done := make(chan struct{})
				r.NoError(client.AppRequest(
					context.Background(),
					ids.EmptyNodeID,
					&dsmr.GetChunkRequest{
						ChunkId: chunk.id[:],
						Expiry:  chunk.Expiry,
					},
					func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
						defer close(done)

						r.NoError(err)

						gotChunks[response.id] = response
					},
				))
				<-done
			}

			for _, chunk := range wantChunks {
				r.Contains(gotChunks, chunk.id)

				gotChunk := gotChunks[chunk.id]
				r.Equal(nodeID, gotChunk.Producer)
				r.Equal(chunk.Expiry, gotChunk.Expiry)
				r.Equal(chunk.Beneficiary, gotChunk.Beneficiary)
				r.ElementsMatch(chunk.Txs, gotChunk.Txs)
				// TODO verify
				r.NotEmpty(chunk.Signer)
				r.NotEmpty(chunk.Signature)
			}
		})
	}
}

// Node should be willing to sign valid chunks
func TestNode_GetChunkSignature_SignValidChunk(t *testing.T) {
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

			networkID := uint32(123)
			chainID := ids.Empty
			sk1, err := bls.NewSecretKey()
			r.NoError(err)
			pk1 := bls.PublicFromSecretKey(sk1)
			signer1 := warp.NewSigner(sk1, networkID, chainID)

			node1, err := New[tx](
				ids.GenerateTestNodeID(),
				networkID,
				chainID,
				pk1,
				signer1,
				tt.verifier,
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				nil,
				logging.NoLog{},
				trace.Noop,
				newTestingChainIndexer(),
			)
			r.NoError(err)

			client := NewGetChunkSignatureClient(
				networkID,
				chainID,
				p2ptest.NewClient(
					t,
					context.Background(),
					node1.GetChunkSignatureHandler,
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
			)

			sk2, err := bls.NewSecretKey()
			r.NoError(err)
			pk2 := bls.PublicFromSecretKey(sk2)
			signer2 := warp.NewSigner(sk2, networkID, chainID)
			node2, err := New[tx](
				ids.GenerateTestNodeID(),
				networkID,
				chainID,
				pk2,
				signer2,
				tt.verifier,
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				nil,
				logging.NoLog{},
				trace.Noop,
				newTestingChainIndexer(),
			)
			r.NoError(err)
			chunk, err := node2.BuildChunk(
				context.Background(),
				[]tx{{ID: ids.Empty, Expiry: 123}},
				123,
				codec.Address{123},
			)
			r.NoError(err)

			done := make(chan struct{})
			onResponse := func(_ context.Context, _ ids.NodeID, response *dsmr.GetChunkSignatureResponse, err error) {
				defer close(done)

				r.ErrorIs(err, tt.wantErr)
				if err != nil {
					return
				}

				pChain := &validatorstest.State{
					T: t,
					GetSubnetIDF: func(context.Context, ids.ID) (ids.ID, error) {
						return ids.Empty, nil
					},
					GetValidatorSetF: func(
						context.Context,
						uint64,
						ids.ID,
					) (map[ids.NodeID]*validators.GetValidatorOutput, error) {
						return map[ids.NodeID]*validators.GetValidatorOutput{
							node1.nodeID: {
								NodeID:    node1.nodeID,
								PublicKey: pk1,
								Weight:    1,
							},
							node2.nodeID: {
								NodeID:    node2.nodeID,
								PublicKey: pk2,
								Weight:    1,
							},
						}, nil
					},
				}

				signature := warp.BitSetSignature{
					Signers:   getSignerBitSet(t, pChain, node1.nodeID).Bytes(),
					Signature: [bls.SignatureLen]byte{},
				}

				copy(signature.Signature[:], response.Signature)

				msg, err := warp.NewUnsignedMessage(networkID, chainID, chunk.bytes)
				r.NoError(err)
				r.NoError(signature.Verify(
					context.Background(),
					msg,
					networkID,
					pChain,
					0,
					1,
					2,
				))
			}

			packer := wrappers.Packer{MaxSize: MaxMessageSize}
			r.NoError(codec.LinearCodec.MarshalInto(chunk, &packer))

			r.NoError(client.AppRequest(
				context.Background(),
				ids.EmptyNodeID,
				&dsmr.GetChunkSignatureRequest{Chunk: packer.Bytes},
				onResponse,
			))

			<-done
		})
	}
}

// Node should not sign duplicate chunks that have already been accepted
func TestNode_GetChunkSignature_DuplicateChunk(t *testing.T) {
	r := require.New(t)

	networkID := uint32(123)
	chainID := ids.Empty
	sk, err := bls.NewSecretKey()
	r.NoError(err)
	pk := bls.PublicFromSecretKey(sk)
	signer := warp.NewSigner(sk, networkID, chainID)
	node, err := New[tx](
		ids.EmptyNodeID,
		networkID,
		chainID,
		pk,
		signer,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	client := NewGetChunkSignatureClient(
		networkID,
		chainID,
		p2ptest.NewClient(
			t,
			context.Background(),
			node.GetChunkSignatureHandler,
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		))

	// Accept a chunk
	r.NoError(err)
	chunk, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.Empty, Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)
	blk, err := node.BuildBlock(
		context.Background(),
		Block{
			ParentID: ids.GenerateTestID(),
			Hght:     0,
			Tmstmp:   1,
		},
		2,
	)
	r.NoError(err)
	r.NoError(node.Accept(context.Background(), blk))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ *dsmr.GetChunkSignatureResponse, err error) {
		defer close(done)

		r.ErrorIs(err, ErrDuplicateChunk)
	}

	packer := wrappers.Packer{MaxSize: MaxMessageSize}
	r.NoError(codec.LinearCodec.MarshalInto(chunk, &packer))

	r.NoError(client.AppRequest(
		context.Background(),
		ids.EmptyNodeID,
		&dsmr.GetChunkSignatureRequest{Chunk: packer.Bytes},
		onResponse,
	))

	<-done
}

// Nodes must persist chunks that they sign from other nodes
func TestGetChunkSignature_PersistAttestedBlocks(t *testing.T) {
	r := require.New(t)

	networkID := uint32(23)
	chainID := ids.Empty
	sk1, err := bls.NewSecretKey()
	r.NoError(err)
	pk1 := bls.PublicFromSecretKey(sk1)
	signer1 := warp.NewSigner(sk1, networkID, chainID)
	node1, err := New[tx](
		ids.EmptyNodeID,
		networkID,
		chainID,
		pk1,
		signer1,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	sk2, err := bls.NewSecretKey()
	r.NoError(err)
	pk2 := bls.PublicFromSecretKey(sk2)
	signer2 := warp.NewSigner(sk2, networkID, chainID)
	node2, err := New[tx](
		ids.EmptyNodeID,
		networkID,
		chainID,
		pk2,
		signer2,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			node1.GetChunkHandler,
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			node1.GetChunkSignatureHandler,
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			node1.ChunkCertificateGossipHandler,
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		[]Validator{{NodeID: node1.nodeID}},
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	chunk, err := node2.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.Empty, Expiry: 1}},
		1,
		codec.Address{123},
	)
	r.NoError(err)

	// Keep trying to build a block until we hear about the newly generated
	// chunk cert
	var blk Block
	for {
		blk, err = node1.BuildBlock(
			context.Background(),
			Block{
				ParentID: ids.Empty,
				Hght:     0,
				Tmstmp:   0,
			},
			1,
		)
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	r.NoError(node1.Accept(context.Background(), blk))

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		node1.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
		defer close(done)
		r.NoError(err)
		r.Equal(chunk, response)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		ids.EmptyNodeID,
		&dsmr.GetChunkRequest{
			ChunkId: chunk.id[:],
			Expiry:  chunk.Expiry,
		},
		onResponse,
	))
	<-done
}

func TestNode_NewBlock_IncludesChunkCerts(t *testing.T) {
	type chunk struct {
		txs         []tx
		expiry      int64
		beneficiary codec.Address
	}

	tests := []struct {
		name      string
		chunks    []chunk
		parent    Block
		timestamp int64
		wantErr   error
	}{
		{
			name: "no chunk certs",
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 2,
			// TODO should we be able to build empty blocks?
			wantErr: ErrNoAvailableChunkCerts,
		},
		{
			name: "timestamp equal to parent",
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 1,
			wantErr:   ErrTimestampNotMonotonicallyIncreasing,
		},
		{
			name: "timestamp older than parent",
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 0,
			wantErr:   ErrTimestampNotMonotonicallyIncreasing,
		},
		{
			name: "expired chunk cert",
			chunks: []chunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: 2,
				},
			},
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 3,
			wantErr:   ErrNoAvailableChunkCerts,
		},
		{
			name: "multiple expired chunk certs",
			chunks: []chunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry: 1,
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry: 2,
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry: 3,
				},
			},
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 5,
			wantErr:   ErrNoAvailableChunkCerts,
		},
		{
			name: "single chunk cert",
			chunks: []chunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: 2,
				},
			},
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 2,
		},
		{
			name: "multiple chunk certs",
			chunks: []chunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: 2,
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: 2,
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 2,
						},
					},
					expiry: 2,
				},
			},
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     1,
				Tmstmp:   1,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 2,
		},
		{
			name: "one expired and one pending chunk cert",
			chunks: []chunk{
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry: 1,
				},
				{
					txs: []tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 3,
						},
					},
					expiry: 3,
				},
			},
			parent: Block{
				ParentID: ids.GenerateTestID(),
				Hght:     2,
				Tmstmp:   2,
				blkID:    ids.GenerateTestID(),
			},
			timestamp: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			networkID := uint32(123)
			chainID := ids.Empty
			sk, err := bls.NewSecretKey()
			r.NoError(err)
			pk := bls.PublicFromSecretKey(sk)
			signer := warp.NewSigner(sk, networkID, chainID)

			node, err := New[tx](
				ids.EmptyNodeID,
				networkID,
				chainID,
				pk,
				signer,
				NoVerifier[tx]{},
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				nil,
				logging.NoLog{},
				trace.Noop,
				newTestingChainIndexer(),
			)
			r.NoError(err)

			wantChunks := make([]Chunk[tx], 0)
			for _, chunk := range tt.chunks {
				chunk, err := node.BuildChunk(
					context.Background(),
					chunk.txs,
					chunk.expiry,
					chunk.beneficiary,
				)
				r.NoError(err)

				// Only expect chunks that have not expired
				if chunk.Expiry < tt.timestamp {
					continue
				}

				wantChunks = append(wantChunks, chunk)
			}
			r.NoError(node.Accept(context.Background(), tt.parent))
			blk, err := node.BuildBlock(context.Background(), tt.parent, tt.timestamp)
			r.ErrorIs(err, tt.wantErr)
			if err != nil {
				return
			}

			r.Equal(tt.parent.GetID(), blk.ParentID)
			r.Equal(tt.parent.Hght+1, blk.Hght)
			r.Greater(blk.Tmstmp, tt.parent.Tmstmp)
			r.NotEmpty(blk.GetID())
			r.Len(blk.ChunkCerts, len(wantChunks))

			for _, chunk := range wantChunks {
				found := false
				for _, chunkCert := range blk.ChunkCerts {
					if chunkCert.ChunkID == chunk.id {
						found = true
					}
				}
				r.True(found)
			}
		})
	}
}

// TestDuplicateChunksElimination tests that duplicate chunks that have appeared before are getting correctly eliminated.
func TestDuplicateChunksElimination(t *testing.T) {
	r := require.New(t)

	networkID := uint32(123)
	chainID := ids.Empty
	sk, err := bls.NewSecretKey()
	r.NoError(err)
	pk := bls.PublicFromSecretKey(sk)
	signer := warp.NewSigner(sk, networkID, chainID)

	node, err := New[tx](
		ids.EmptyNodeID,
		networkID,
		chainID,
		pk,
		signer,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	blk := Block{
		ParentID: ids.GenerateTestID(),
		Hght:     1,
		Tmstmp:   1,
		blkID:    ids.GenerateTestID(),
	}
	r.NoError(node.Accept(context.Background(), blk))

	chunk, err := node.BuildChunk(
		context.Background(),
		[]tx{
			{
				ID:     ids.GenerateTestID(),
				Expiry: 4,
			},
		},
		4,
		codec.Address{},
	)
	r.NoError(err)
	chunkCert := &ChunkCertificate{
		ChunkID:   chunk.GetID(),
		Expiry:    chunk.GetExpiry(),
		Signature: NoVerifyChunkSignature{},
	}

	blk = Block{
		ParentID: ids.GenerateTestID(),
		Hght:     2,
		Tmstmp:   2,
		blkID:    ids.GenerateTestID(),
		ChunkCerts: []*ChunkCertificate{
			chunkCert,
		},
	}
	r.NoError(node.Accept(context.Background(), blk))

	r.NoError(node.storage.AddLocalChunkWithCert(chunk, chunkCert))
	_, err = node.BuildBlock(context.Background(), blk, 3)
	r.ErrorIs(err, ErrAllChunkCertsDuplicate)

	// make sure that it's not the case with any other chunk.
	chunk, err = node.BuildChunk(
		context.Background(),
		[]tx{
			{
				ID:     ids.GenerateTestID(),
				Expiry: 4,
			},
		},
		4,
		codec.Address{},
	)
	r.NoError(err)
	chunkCert = &ChunkCertificate{
		ChunkID:   chunk.GetID(),
		Expiry:    chunk.GetExpiry(),
		Signature: NoVerifyChunkSignature{},
	}
	r.NoError(node.Accept(context.Background(), blk))

	r.NoError(node.storage.AddLocalChunkWithCert(chunk, chunkCert))
	_, err = node.BuildBlock(context.Background(), blk, 3)
	r.NoError(err)
}

func TestNode_Execute_Chunks(t *testing.T) {
	r := require.New(t)
	networkID := uint32(123)
	chainID := ids.Empty
	sk1, err := bls.NewSecretKey()
	r.NoError(err)
	pk := bls.PublicFromSecretKey(sk1)
	signer := warp.NewSigner(sk1, networkID, chainID)
	r.NoError(err)

	makeChunkCert := func(chunk Chunk[tx]) *ChunkCertificate {
		return &ChunkCertificate{
			ChunkID:   chunk.GetID(),
			Expiry:    chunk.GetExpiry(),
			Signature: NoVerifyChunkSignature{},
		}
	}
	initChunks := func(node *Node[tx]) []Chunk[tx] {
		var chunks []Chunk[tx]
		for expiry := int64(0); expiry < 5; expiry++ {
			chunk, err := node.BuildChunk(
				context.Background(),
				[]tx{
					{
						ID:     ids.GenerateTestID(),
						Expiry: expiry,
					},
				},
				expiry,
				codec.Address{},
			)
			r.NoError(err)
			chunks = append(chunks, chunk)

		}
		return chunks
	}
	testCases := []struct {
		name           string
		parentBlocks   [][]int // for each parent, a list of the chunks included.
		chunks         []int
		timestamp      int64
		executeWantErr error
		buildWantErr   error
	}{
		{
			name:           "three empty blocks",
			parentBlocks:   [][]int{{}, {}, {}},
			chunks:         []int{},
			timestamp:      4,
			executeWantErr: nil,
		},
		{
			name:           "three blocks, unique chunks",
			parentBlocks:   [][]int{{1}, {2}, {3}},
			chunks:         []int{},
			timestamp:      4,
			executeWantErr: nil,
		},
		{
			name:           "two blocks one duplicate chunk",
			parentBlocks:   [][]int{{0, 1}, {1, 2}},
			chunks:         []int{},
			timestamp:      4,
			executeWantErr: validitywindow.ErrDuplicateContainer,
		},
		{
			name:           "one block duplicate chunks",
			parentBlocks:   [][]int{{1, 1}},
			chunks:         []int{},
			timestamp:      4,
			executeWantErr: validitywindow.ErrDuplicateContainer,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			indexer := newTestingChainIndexer()
			node, err := New[tx](
				ids.EmptyNodeID,
				networkID,
				chainID,
				pk,
				signer,
				NoVerifier[tx]{},
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				p2ptest.NewClient(
					t,
					context.Background(),
					&p2p.NoOpHandler{},
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				nil,
				logging.NoLog{},
				trace.Noop,
				indexer,
			)
			r.NoError(err)

			chunks := initChunks(node)

			// initialize node history.
			var parentBlk Block
			for blockNum, chunkList := range testCase.parentBlocks {
				blk := Block{
					ParentID: parentBlk.GetID(),
					Hght:     uint64(blockNum),
					Tmstmp:   int64(blockNum),
					blkID:    ids.GenerateTestID(),
				}
				for _, chunkIndex := range chunkList {
					blk.ChunkCerts = append(blk.ChunkCerts, makeChunkCert(chunks[chunkIndex]))
				}
				if blockNum > 0 {
					r.ErrorIs(node.Execute(context.Background(), parentBlk, blk), testCase.executeWantErr)
				}
				r.NoError(node.Accept(context.Background(), blk))
				indexer.set(blk.GetID(), blk)
				parentBlk = blk
			}
			// feed the chunks into the storage and build a block.
			for _, chunkIdx := range testCase.chunks {
				r.NoError(node.storage.AddLocalChunkWithCert(chunks[chunkIdx], makeChunkCert(chunks[chunkIdx])))
			}
			_, err = node.BuildBlock(context.Background(), parentBlk, testCase.timestamp)
			r.ErrorIs(err, testCase.buildWantErr)
		})
	}
}

// Nodes should request chunks referenced in accepted blocks
func TestAccept_RequestReferencedChunks(t *testing.T) {
	r := require.New(t)

	networkID := uint32(123)
	chainID := ids.Empty
	sk1, err := bls.NewSecretKey()
	r.NoError(err)
	pk1 := bls.PublicFromSecretKey(sk1)
	signer1 := warp.NewSigner(sk1, networkID, chainID)
	r.NoError(err)
	node1, err := New[tx](
		ids.GenerateTestNodeID(),
		networkID,
		chainID,
		pk1,
		signer1,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		nil,
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)

	chunk, err := node1.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 1}},
		1,
		codec.Address{123},
	)
	r.NoError(err)
	blk, err := node1.BuildBlock(
		context.Background(),
		Block{
			ParentID: ids.GenerateTestID(),
			Hght:     0,
			Tmstmp:   0,
		},
		1)
	r.NoError(err)
	r.NoError(node1.Accept(context.Background(), blk))

	sk2, err := bls.NewSecretKey()
	r.NoError(err)
	pk2 := bls.PublicFromSecretKey(sk2)
	signer2 := warp.NewSigner(sk2, networkID, chainID)
	node2, err := New[tx](
		ids.GenerateTestNodeID(),
		networkID,
		chainID,
		pk2,
		signer2,
		NoVerifier[tx]{},
		p2ptest.NewClient(
			t,
			context.Background(),
			node1.GetChunkHandler,
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		p2ptest.NewClient(
			t,
			context.Background(),
			&p2p.NoOpHandler{},
			ids.EmptyNodeID,
			ids.EmptyNodeID,
		),
		[]Validator{{NodeID: node1.nodeID}},
		logging.NoLog{},
		trace.Noop,
		newTestingChainIndexer(),
	)
	r.NoError(err)
	r.NoError(node2.Accept(context.Background(), blk))

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		node2.GetChunkHandler,
		ids.EmptyNodeID,
		ids.EmptyNodeID,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
		defer close(done)
		r.NoError(err)
		r.Equal(chunk, response)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		ids.EmptyNodeID,
		&dsmr.GetChunkRequest{
			ChunkId: chunk.id[:],
			Expiry:  chunk.Expiry,
		},
		onResponse,
	))
	<-done
}

func getSignerBitSet(t *testing.T, pChain validators.State, nodeIDs ...ids.NodeID) set.Bits {
	validators, _, err := warp.GetCanonicalValidatorSet(
		context.Background(),
		pChain,
		0,
		ids.Empty,
	)
	require.NoError(t, err)

	signers := set.Of(nodeIDs...)
	signerBitSet := set.NewBits()
	for i, v := range validators {
		for _, nodeID := range v.NodeIDs {
			if signers.Contains(nodeID) {
				signerBitSet.Add(i)
				break
			}
		}
	}

	return signerBitSet
}

type tx struct {
	ID      ids.ID        `serialize:"true"`
	Expiry  int64         `serialize:"true"`
	Sponsor codec.Address `serialize:"true"`
}

func (t tx) GetID() ids.ID {
	return t.ID
}

func (t tx) GetExpiry() int64 {
	return t.Expiry
}

func (t tx) GetSponsor() codec.Address {
	return t.Sponsor
}

type failVerifier struct{}

func (failVerifier) Verify(Chunk[tx]) error {
	return errors.New("fail")
}
