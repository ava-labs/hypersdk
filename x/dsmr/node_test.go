// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/proto/pb/sdk"
	"github.com/ava-labs/avalanchego/snow/validators/validatorstest"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"

	snowValidators "github.com/ava-labs/avalanchego/snow/validators"
)

const networkID = uint32(123)

var (
	_ Tx           = (*tx)(nil)
	_ Verifier[tx] = (*failVerifier)(nil)

	chainID = ids.Empty
)

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

			node := newNode(t)
			chunk, _, err := node.BuildChunk(
				context.Background(),
				tt.txs,
				tt.expiry,
				tt.beneficiary,
			)
			r.ErrorIs(err, tt.wantErr)
			if err != nil {
				return
			}

			r.Equal(node.ID, chunk.Producer)
			r.Equal(tt.beneficiary, chunk.Beneficiary)
			r.Equal(tt.expiry, chunk.Expiry)
			r.ElementsMatch(tt.txs, chunk.Txs)

			wantPkBytes := [bls.PublicKeyLen]byte{}
			copy(wantPkBytes[:], bls.PublicKeyToCompressedBytes(node.PublicKey))
			r.Equal(wantPkBytes, chunk.Signer)

			packer := &wrappers.Packer{MaxSize: MaxMessageSize}
			r.NoError(codec.LinearCodec.MarshalInto(chunk.UnsignedChunk, packer))
			msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
			r.NoError(err)
			wantSignature, err := node.Signer.Sign(msg)
			r.NoError(err)
			r.Equal(wantSignature, chunk.Signature[:])
		})
	}
}

// Tests that chunks referenced in accepted blocks are available over p2p
func TestNode_GetChunk_AvailableChunk(t *testing.T) {
	r := require.New(t)

	nodes := newNodes(t, 2)
	node := nodes[0]

	chunk, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	blk, err := node.BuildBlock(
		Block[tx]{
			ParentID:  ids.GenerateTestID(),
			Height:    0,
			Timestamp: 1,
		},
		2,
	)
	r.NoError(err)
	r.NoError(node.Accept(context.Background(), blk))

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node.ID,
		node.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
		defer close(done)
		r.NoError(err)
		r.Equal(chunk, response)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		node.ID,
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

	node := newNode(t)
	chunk, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node.ID,
		node.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ Chunk[tx], err error) {
		defer close(done)

		r.ErrorIs(err, ErrChunkNotAvailable)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		node.ID,
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

	node := newNode(t)
	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node.ID,
		node.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ Chunk[tx], err error) {
		defer close(done)

		r.ErrorIs(err, ErrChunkNotAvailable)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		node.ID,
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

			node := newNode(t)

			// Build some chunks
			wantChunks := make([]Chunk[tx], 0)
			for _, args := range tt.availableChunks {
				chunk, _, err := node.BuildChunk(
					context.Background(),
					args.txs,
					args.expiry,
					args.beneficiary,
				)
				r.NoError(err)

				wantChunks = append(wantChunks, chunk)
			}

			block, err := node.BuildBlock(
				Block[tx]{
					ParentID:  ids.GenerateTestID(),
					Height:    0,
					Timestamp: 1,
				},
				2,
			)
			r.NoError(err)
			r.NoError(node.Accept(context.Background(), block))

			client := NewGetChunkClient[tx](p2ptest.NewClient(
				t,
				context.Background(),
				ids.EmptyNodeID,
				p2p.NoOpHandler{},
				node.ID,
				node.GetChunkHandler,
			))

			// Node must serve GetChunk requests for chunks that it accepted
			gotChunks := make(map[ids.ID]Chunk[tx], 0)
			for _, chunk := range wantChunks {
				done := make(chan struct{})
				r.NoError(client.AppRequest(
					context.Background(),
					node.ID,
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
				r.Equal(node.ID, gotChunk.Producer)
				r.Equal(chunk.Expiry, gotChunk.Expiry)
				r.Equal(chunk.Beneficiary, gotChunk.Beneficiary)
				r.ElementsMatch(chunk.Txs, gotChunk.Txs)

				wantPkBytes := [bls.PublicKeyLen]byte{}
				copy(wantPkBytes[:], bls.PublicKeyToCompressedBytes(node.PublicKey))
				r.Equal(wantPkBytes, chunk.Signer)

				packer := &wrappers.Packer{MaxSize: MaxMessageSize}
				r.NoError(codec.LinearCodec.MarshalInto(chunk.UnsignedChunk, packer))
				msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
				r.NoError(err)
				wantSignature, err := node.Signer.Sign(msg)
				r.NoError(err)
				r.Equal(wantSignature, chunk.Signature[:])
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

			nodeID := ids.GenerateTestNodeID()
			sk, err := bls.NewSecretKey()
			r.NoError(err)
			pk := bls.PublicFromSecretKey(sk)
			signer := warp.NewSigner(sk, networkID, chainID)

			validators := []Validator{
				{
					NodeID:    nodeID,
					Weight:    1,
					PublicKey: pk,
				},
			}

			chunkStorage, err := NewChunkStorage[tx](tt.verifier, memdb.New())
			r.NoError(err)

			node, err := New[tx](
				logging.NoLog{},
				nodeID,
				networkID,
				chainID,
				pk,
				signer,
				chunkStorage,
				p2p.NoOpHandler{},
				acp118.NewHandler(
					ChunkSignatureRequestVerifier[tx]{
						verifier: tt.verifier,
						storage:  chunkStorage,
					},
					signer,
				),
				p2p.NoOpHandler{},
				p2ptest.NewSelfClient(
					t,
					context.Background(),
					nodeID,
					&p2p.NoOpHandler{},
				),
				p2ptest.NewSelfClient(
					t,
					context.Background(),
					nodeID,
					&p2p.NoOpHandler{},
				),
				p2ptest.NewSelfClient(
					t,
					context.Background(),
					nodeID,
					&p2p.NoOpHandler{},
				),
				validators,
				Block[tx]{
					ParentID:  ids.GenerateTestID(),
					Height:    0,
					Timestamp: 0,
					blkID:     ids.GenerateTestID(),
				},
				1,
				1,
			)
			r.NoError(err)

			chunk, _, err := node.BuildChunk(
				context.Background(),
				[]tx{{ID: ids.Empty, Expiry: 123}},
				123,
				codec.Address{123},
			)
			r.NoError(err)

			packer := wrappers.Packer{MaxSize: MaxMessageSize}
			r.NoError(codec.LinearCodec.MarshalInto(ChunkReference{
				ChunkID:  chunk.id,
				Producer: chunk.Producer,
				Expiry:   chunk.Expiry,
			}, &packer))
			msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
			r.NoError(err)
			done := make(chan struct{})
			onResponse := func(_ context.Context, _ ids.NodeID, response *sdk.SignatureResponse, err error) {
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
					) (map[ids.NodeID]*snowValidators.GetValidatorOutput, error) {
						return map[ids.NodeID]*snowValidators.GetValidatorOutput{
							node.ID: {
								NodeID:    node.ID,
								PublicKey: node.PublicKey,
								Weight:    1,
							},
						}, nil
					},
				}

				signature := warp.BitSetSignature{
					Signers:   getSignerBitSet(t, pChain, node.ID).Bytes(),
					Signature: [bls.SignatureLen]byte{},
				}

				copy(signature.Signature[:], response.Signature)

				r.NoError(signature.Verify(
					context.Background(),
					msg,
					networkID,
					pChain,
					0,
					1,
					1,
				))
			}

			client := NewGetChunkSignatureClient(
				networkID,
				chainID,
				p2ptest.NewClient(
					t,
					context.Background(),
					ids.EmptyNodeID,
					p2p.NoOpHandler{},
					node.ID,
					node.GetChunkSignatureHandler,
				),
			)
			r.NoError(client.AppRequest(
				context.Background(),
				node.ID,
				&sdk.SignatureRequest{
					Message:       msg.Bytes(),
					Justification: chunk.bytes,
				},
				onResponse,
			))

			<-done
		})
	}
}

// Node should not sign duplicate chunks that have already been accepted
func TestNode_GetChunkSignature_DuplicateChunk(t *testing.T) {
	r := require.New(t)

	node := newNode(t)
	chunk, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.Empty, Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)
	blk, err := node.BuildBlock(
		Block[tx]{
			ParentID:  ids.GenerateTestID(),
			Height:    0,
			Timestamp: 1,
		},
		2,
	)
	r.NoError(err)
	r.NoError(node.Accept(context.Background(), blk))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ *sdk.SignatureResponse, err error) {
		defer close(done)

		r.ErrorIs(err, ErrDuplicateChunk)
	}

	packer := wrappers.Packer{MaxSize: MaxMessageSize}
	r.NoError(codec.LinearCodec.MarshalInto(ChunkReference{
		ChunkID:  chunk.id,
		Producer: chunk.Producer,
		Expiry:   chunk.Expiry,
	}, &packer))
	msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
	r.NoError(err)

	client := NewGetChunkSignatureClient(
		networkID,
		chainID,
		p2ptest.NewClient(
			t,
			context.Background(),
			ids.EmptyNodeID,
			p2p.NoOpHandler{},
			node.ID,
			node.GetChunkSignatureHandler,
		),
	)
	r.NoError(client.AppRequest(
		context.Background(),
		node.ID,
		&sdk.SignatureRequest{
			Message:       msg.Bytes(),
			Justification: chunk.bytes,
		},
		onResponse,
	))

	<-done
}

// Nodes must persist chunks that they sign from other nodes
func TestGetChunkSignature_PersistAttestedBlocks(t *testing.T) {
	r := require.New(t)

	nodes := newNodes(t, 2)
	node1 := nodes[0]
	node2 := nodes[1]

	chunk, _, err := node1.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.Empty, Expiry: 1}},
		1,
		codec.Address{123},
	)
	r.NoError(err)

	// Keep trying to build a block until we hear about the newly generated
	// chunk cert
	var blk Block[tx]
	for {
		blk, err = node2.BuildBlock(
			Block[tx]{
				ParentID:  ids.Empty,
				Height:    0,
				Timestamp: 0,
			},
			1,
		)
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	r.NoError(node2.Accept(context.Background(), blk))

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node2.ID,
		node2.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
		defer close(done)
		r.NoError(err)
		r.Equal(chunk, response)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		node2.ID,
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
		parent    Block[tx]
		timestamp int64
		wantErr   error
	}{
		{
			name: "no chunk certs",
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
			},
			timestamp: 2,
			// TODO should we be able to build empty blocks?
			wantErr: ErrNoAvailableChunkCerts,
		},
		{
			name: "timestamp equal to parent",
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
			},
			timestamp: 1,
			wantErr:   ErrTimestampNotMonotonicallyIncreasing,
		},
		{
			name: "timestamp older than parent",
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
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
							Expiry: 1,
						},
					},
					expiry: 1,
				},
			},
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
			},
			timestamp: 2,
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
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
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
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
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
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
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
			parent: Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    1,
				Timestamp: 1,
				blkID:     ids.GenerateTestID(),
			},
			timestamp: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			node := newNode(t)
			wantChunks := make([]Chunk[tx], 0)
			for _, chunk := range tt.chunks {
				chunk, _, err := node.BuildChunk(
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

			blk, err := node.BuildBlock(tt.parent, tt.timestamp)
			r.ErrorIs(err, tt.wantErr)
			if err != nil {
				return
			}

			r.Equal(tt.parent.GetID(), blk.ParentID)
			r.Equal(tt.parent.Height+1, blk.Height)
			r.Greater(blk.Timestamp, tt.parent.Timestamp)
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

// Nodes should request chunks referenced in accepted blocks
func TestAccept_RequestReferencedChunks(t *testing.T) {
	r := require.New(t)

	nodes := newNodes(t, 2)
	node1 := nodes[0]
	node2 := nodes[1]

	chunk, _, err := node1.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 1}},
		1,
		codec.Address{123},
	)
	r.NoError(err)
	blk, err := node1.BuildBlock(Block[tx]{
		ParentID:  ids.GenerateTestID(),
		Height:    0,
		Timestamp: 0,
	}, 1)
	r.NoError(err)
	r.NoError(node1.Accept(context.Background(), blk))
	r.NoError(node2.Accept(context.Background(), blk))

	client := NewGetChunkClient[tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node2.ID,
		node2.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[tx], err error) {
		defer close(done)
		r.NoError(err)
		r.Equal(chunk, response)
	}

	r.NoError(client.AppRequest(
		context.Background(),
		node2.ID,
		&dsmr.GetChunkRequest{
			ChunkId: chunk.id[:],
			Expiry:  chunk.Expiry,
		},
		onResponse,
	))
	<-done
}

func getSignerBitSet(t *testing.T, pChain snowValidators.State, nodeIDs ...ids.NodeID) set.Bits {
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

func Test_Verify(t *testing.T) {
	r := require.New(t)

	node := newNode(t)
	_, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 1}},
		100,
		codec.Address{123},
	)
	r.NoError(err)

	blk, err := node.BuildBlock(node.LastAccepted, node.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
}

func Test_Verify_BadBlock(t *testing.T) {
	tests := []struct {
		name    string
		blk     func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx]
		wantErr error
	}{
		{
			name: "invalid parent",
			blk: func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   ids.GenerateTestID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockParent,
		},
		{
			name: "invalid height - before parent",
			blk: func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height - 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid height - same as parent",
			blk: func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid height - too far into future",
			blk: func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 2,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid timestamp - before parent",
			blk: func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp - 1,
					ChunkCerts: []*ChunkCertificate[tx]{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "invalid timestamp - same as parent",
			blk: func(chunkCert ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp,
					ChunkCerts: []*ChunkCertificate[tx]{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "invalid timestamp - too far into future",
			blk: func(_ ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + time.Minute.Nanoseconds(),
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "duplicate chunk cert",
			blk: func(_ ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrDuplicateChunkCert,
		},
		{
			name: "nil chunk certs",
			blk: func(_ ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + 1,
				}
			},
			wantErr: ErrEmptyBlock,
		},
		{
			name: "empty chunk certs",
			blk: func(_ ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{},
				}
			},
			wantErr: ErrEmptyBlock,
		},
		{
			name: "missing signature",
			blk: func(_ ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{
						{
							ChunkReference: ChunkReference{
								ChunkID:  ids.GenerateTestID(),
								Producer: ids.GenerateTestNodeID(),
								Expiry:   1,
							},
							Signature: nil,
						},
					},
				}
			},
			wantErr: ErrMissingChunkSignature,
		},
		{
			name: "invalid signature",
			blk: func(_ ChunkCertificate[tx], parent Block[tx]) Block[tx] {
				return Block[tx]{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate[tx]{
						{
							ChunkReference: ChunkReference{
								ChunkID:  ids.GenerateTestID(),
								Producer: ids.GenerateTestNodeID(),
								Expiry:   1,
							},
							Signature: &warp.BitSetSignature{
								Signers:   set.NewBits(1, 2, 3).Bytes(),
								Signature: [96]byte{1, 2, 3},
							},
						},
					},
				}
			},
			wantErr: ErrInvalidWarpSignature,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			node := newNode(t)
			_, _, err := node.BuildChunk(
				context.Background(),
				[]tx{{ID: ids.GenerateTestID(), Expiry: 1}},
				100,
				codec.Address{123},
			)
			r.NoError(err)

			parentBlk, err := node.BuildBlock(
				node.LastAccepted,
				node.LastAccepted.Timestamp+1,
			)
			r.NoError(err)
			r.NoError(node.Verify(context.Background(), node.LastAccepted, parentBlk))
			r.NoError(node.Accept(context.Background(), parentBlk))

			_, chunkCert, err := node.BuildChunk(
				context.Background(),
				[]tx{{ID: ids.GenerateTestID(), Expiry: 2}},
				100,
				codec.Address{123},
			)
			r.NoError(err)
			r.ErrorIs(node.Verify(
				context.Background(),
				parentBlk,
				tt.blk(chunkCert, parentBlk),
			), tt.wantErr)
		})
	}
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

type testNode struct {
	ChunkStorage                  *ChunkStorage[tx]
	GetChunkHandler               p2p.Handler
	ChunkSignatureRequestHandler  p2p.Handler
	ChunkCertificateGossipHandler p2p.Handler
	Sk                            *bls.SecretKey
}

func newNode(t *testing.T) *Node[tx] {
	return newNodes(t, 1)[0]
}

func newNodes(t *testing.T, n int) []*Node[tx] {
	nodes := make([]testNode, 0, n)
	validators := make([]Validator, 0, n)
	for i := 0; i < n; i++ {
		sk, err := bls.NewSecretKey()
		require.NoError(t, err)
		pk := bls.PublicFromSecretKey(sk)
		signer := warp.NewSigner(sk, networkID, chainID)

		chunkStorage, err := NewChunkStorage[tx](NoVerifier[tx]{}, memdb.New())
		require.NoError(t, err)

		getChunkHandler := &GetChunkHandler[tx]{
			storage: chunkStorage,
		}
		chunkSignatureRequestHandler := acp118.NewHandler(ChunkSignatureRequestVerifier[tx]{
			verifier: NoVerifier[tx]{},
			storage:  chunkStorage,
		}, signer)
		chunkCertificateGossipHandler := ChunkCertificateGossipHandler[tx]{
			storage: chunkStorage,
		}

		nodes = append(nodes, testNode{
			ChunkStorage:                  chunkStorage,
			GetChunkHandler:               getChunkHandler,
			ChunkSignatureRequestHandler:  chunkSignatureRequestHandler,
			ChunkCertificateGossipHandler: chunkCertificateGossipHandler,
			Sk:                            sk,
		})

		validators = append(validators, Validator{
			NodeID:    ids.GenerateTestNodeID(),
			Weight:    1,
			PublicKey: pk,
		})
	}

	result := make([]*Node[tx], 0, n)
	for i, n := range nodes {
		getChunkPeers := make(map[ids.NodeID]p2p.Handler)
		chunkSignaturePeers := make(map[ids.NodeID]p2p.Handler)
		chunkCertGossipPeers := make(map[ids.NodeID]p2p.Handler)
		for j := range nodes {
			if i == j {
				continue
			}

			getChunkPeers[validators[j].NodeID] = nodes[j].GetChunkHandler
			chunkSignaturePeers[validators[j].NodeID] = nodes[j].ChunkSignatureRequestHandler
			chunkCertGossipPeers[validators[j].NodeID] = nodes[j].ChunkCertificateGossipHandler
		}

		node, err := New[tx](
			logging.NoLog{},
			validators[i].NodeID,
			networkID,
			chainID,
			validators[i].PublicKey,
			warp.NewSigner(n.Sk, networkID, chainID),
			n.ChunkStorage,
			n.GetChunkHandler,
			n.ChunkSignatureRequestHandler,
			n.ChunkCertificateGossipHandler,
			p2ptest.NewClientWithPeers(
				t,
				context.Background(),
				validators[i].NodeID,
				n.GetChunkHandler,
				getChunkPeers,
			),
			p2ptest.NewClientWithPeers(
				t,
				context.Background(),
				validators[i].NodeID,
				n.ChunkSignatureRequestHandler,
				chunkSignaturePeers,
			),
			p2ptest.NewClientWithPeers(
				t,
				context.Background(),
				validators[i].NodeID,
				n.ChunkCertificateGossipHandler,
				chunkCertGossipPeers,
			),
			validators,
			Block[tx]{
				ParentID:  ids.GenerateTestID(),
				Height:    0,
				Timestamp: 0,
				blkID:     ids.GenerateTestID(),
			},
			1,
			1,
		)
		require.NoError(t, err)

		result = append(result, node)
	}

	return result
}
