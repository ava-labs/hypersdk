// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/acp118"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/proto/pb/sdk"
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

	snowValidators "github.com/ava-labs/avalanchego/snow/validators"
)

const (
	networkID                            = uint32(123)
	testingDefaultValidityWindowDuration = time.Duration(5)
)

var (
	_ Tx                                               = (*tx)(nil)
	_ Verifier[tx]                                     = (*failVerifier)(nil)
	_ validitywindow.ChainIndex[*emapChunkCertificate] = (*testValidityWindowChainIndex)(nil)

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

			node := newTestNode(t)
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

	nodes, _ := newNodes(t, 2)
	node := nodes[0]

	chunk, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
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

func TestIndexerMissingBlock(t *testing.T) {
	r := require.New(t)

	node := newTestNode(t)
	_, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, 3)
	r.NoError(err)

	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
	r.NoError(node.Accept(context.Background(), blk))

	_, _, err = node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	blkNext, err := node.BuildBlock(context.Background(), node.LastAccepted, 4)
	r.NoError(err)

	r.ErrorIs(node.Verify(context.Background(), node.LastAccepted, blkNext), database.ErrNotFound)
}

// Tests that pending chunks are not available over p2p
func TestNode_GetChunk_PendingChunk(t *testing.T) {
	r := require.New(t)

	node := newTestNode(t)
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

	node := newTestNode(t)
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

			node := newTestNode(t)

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

			blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
			r.NoError(err)
			r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
			r.NoError(node.Accept(context.Background(), blk))

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
				trace.Noop,
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
				Block{
					ParentID:  ids.GenerateTestID(),
					Height:    0,
					Timestamp: 0,
					blkID:     ids.GenerateTestID(),
				},
				1,
				1,
				newTestValidityWindowChainIndex(),
				testingDefaultValidityWindowDuration,
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

	node := newTestNode(t)
	chunk, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.Empty, Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)
	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
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

	nodes, _ := newNodes(t, 2)
	node1 := nodes[0]
	node2 := nodes[1]

	chunk, _, err := node1.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.Empty, Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)

	// Keep trying to build a block until we hear about the newly generated
	// chunk cert
	var blk Block
	for {
		blk, err = node2.BuildBlock(context.Background(), node2.LastAccepted, node2.LastAccepted.Timestamp+1)
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	r.NoError(node2.Verify(context.Background(), node2.LastAccepted, blk))
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
		chunks    func(parent Block) []chunk
		timestamp func(parent Block) int64
		wantErr   error
	}{
		{
			name: "no chunk certs",
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 1
			},
			// TODO should we be able to build empty blocks?
			wantErr: ErrNoAvailableChunkCerts,
		},
		{
			name: "timestamp equal to parent",
			timestamp: func(parent Block) int64 {
				return parent.Timestamp
			},
			wantErr: ErrTimestampNotMonotonicallyIncreasing,
		},
		{
			name: "timestamp older than parent",
			timestamp: func(parent Block) int64 {
				return parent.Timestamp - 1
			},
			wantErr: ErrTimestampNotMonotonicallyIncreasing,
		},
		{
			name: "expired chunk cert",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1,
							},
						},
						expiry: parent.Timestamp + 1,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 100
			},
			wantErr: ErrNoAvailableChunkCerts,
		},
		{
			name: "multiple expired chunk certs",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1,
							},
						},
						expiry: parent.Timestamp + 1,
					},
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 2,
							},
						},
						expiry: parent.Timestamp + 2,
					},
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 3,
							},
						},
						expiry: parent.Timestamp + 3,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 100
			},
			wantErr: ErrNoAvailableChunkCerts,
		},
		{
			name: "single chunk cert",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1,
							},
						},
						expiry: parent.Timestamp + 1,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 1
			},
		},
		{
			name: "multiple chunk certs",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1_000,
							},
						},
						expiry: parent.Timestamp + 1_000,
					},
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 2_000,
							},
						},
						expiry: parent.Timestamp + 2_000,
					},
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 3_000,
							},
						},
						expiry: parent.Timestamp + 3_000,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 100
			},
		},
		{
			name: "one expired and one pending chunk cert",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1_000,
							},
						},
						expiry: parent.Timestamp + 1_000,
					},
					{
						txs: []tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1,
							},
						},
						expiry: parent.Timestamp + 1,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 100
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			node := newTestNode(t)

			timestamp := tt.timestamp(node.LastAccepted)
			chunks := []chunk{}

			if tt.chunks != nil {
				chunks = tt.chunks(node.LastAccepted)
			}

			wantChunks := make([]Chunk[tx], 0)
			for _, chunk := range chunks {
				chunk, _, err := node.BuildChunk(
					context.Background(),
					chunk.txs,
					chunk.expiry,
					chunk.beneficiary,
				)
				r.NoError(err)

				// Only expect chunks that have not expired
				if chunk.Expiry < timestamp {
					continue
				}

				wantChunks = append(wantChunks, chunk)
			}

			blk, err := node.BuildBlock(context.Background(), node.LastAccepted, timestamp)
			r.ErrorIs(err, tt.wantErr)
			if err != nil {
				return
			}

			r.Equal(node.LastAccepted.GetID(), blk.ParentID)
			r.Equal(node.LastAccepted.Height+1, blk.Height)
			r.Greater(blk.Timestamp, node.LastAccepted.Timestamp)
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

	node := newTestNode(t)

	_, _, err := node.BuildChunk(
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

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, 2)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
	r.NoError(node.Accept(context.Background(), blk))

	_, err = node.BuildBlock(context.Background(), blk, 3)
	r.ErrorIs(err, ErrNoAvailableChunkCerts)

	// make sure that it's not the case with any other chunk.
	_, _, err = node.BuildChunk(
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
	r.NoError(node.Accept(context.Background(), blk))

	_, err = node.BuildBlock(context.Background(), blk, 3)
	r.NoError(err)
}

func TestNode_Verify_Chunks(t *testing.T) {
	tests := []struct {
		name                       string
		parentBlocks               [][]int // for each parent, a list of the chunks included.
		chunks                     []int
		timestamp                  int64
		verifyWantErr              error
		buildWantErr               error
		validalidityWindowDuration time.Duration
	}{
		{
			name:          "three blocks, unique chunks",
			parentBlocks:  [][]int{{1}, {2}},
			chunks:        []int{4},
			timestamp:     4,
			verifyWantErr: nil,
		},
		{
			name:          "two blocks one duplicate chunk",
			parentBlocks:  [][]int{{0, 2}},
			chunks:        []int{2, 4},
			timestamp:     3,
			verifyWantErr: validitywindow.ErrDuplicateContainer,
			buildWantErr:  nil, // build would filter out duplicate chunks, hence no error.
		},
		{
			name:          "one block duplicate chunks",
			parentBlocks:  [][]int{},
			chunks:        []int{2, 2},
			timestamp:     2,
			verifyWantErr: validitywindow.ErrDuplicateContainer,
			buildWantErr:  nil, // build would filter out duplicate chunks, hence no error.
		},
		{
			name:          "three blocks non consecutive duplicate chunks",
			parentBlocks:  [][]int{{3}, {2}},
			chunks:        []int{3},
			timestamp:     4,
			verifyWantErr: validitywindow.ErrDuplicateContainer,
			buildWantErr:  ErrNoAvailableChunkCerts,
		},
		{
			name:          "monotonic timestamping",
			parentBlocks:  [][]int{},
			chunks:        []int{1},
			timestamp:     0,
			verifyWantErr: ErrInvalidBlockHeight,
			buildWantErr:  ErrTimestampNotMonotonicallyIncreasing,
		},
		{
			name:          "empty block",
			parentBlocks:  [][]int{{1}, {2}, {3}, {4}},
			chunks:        []int{},
			timestamp:     6,
			verifyWantErr: ErrEmptyBlock,
			buildWantErr:  ErrNoAvailableChunkCerts,
		},
		{
			name:                       "three blocks non consecutive duplicate chunks outside validity window",
			parentBlocks:               [][]int{{4}, {5}},
			chunks:                     []int{4},
			timestamp:                  4,
			verifyWantErr:              validitywindow.ErrDuplicateContainer, // this isn't ideal, since it would disqualify a duplicate chunk even when it's outside the validity window. However, it default to the correct direction.
			buildWantErr:               ErrNoAvailableChunkCerts,
			validalidityWindowDuration: 1,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			r := require.New(t)
			validationWindow := testCase.validalidityWindowDuration
			if validationWindow == 0 {
				validationWindow = testingDefaultValidityWindowDuration
			}

			var node *Node[tx]
			nodes, indexer := newNodes(t, 1)
			node = nodes[0]
			node.validityWindowDuration = validationWindow

			// initialize node history.
			parentBlk := node.LastAccepted
			for _, chunkList := range testCase.parentBlocks {
				blk := Block{
					ParentID:  parentBlk.GetID(),
					Height:    uint64(int(node.LastAccepted.Height) + 1),
					Timestamp: int64(int(node.LastAccepted.Timestamp) + 1),
					blkID:     ids.GenerateTestID(),
				}

				for _, chunkExpiry := range chunkList {
					_, chunkCert, err := node.BuildChunk(
						context.Background(),
						[]tx{
							{
								ID:     ids.Empty.Prefix(uint64(chunkExpiry)),
								Expiry: int64(chunkExpiry),
							},
						},
						int64(chunkExpiry),
						codec.Address{},
					)
					r.NoError(err)
					blk.ChunkCerts = append(blk.ChunkCerts, &chunkCert)
				}

				r.NoError(node.Verify(context.Background(), parentBlk, blk))

				r.NoError(node.Accept(context.Background(), blk))
				indexer.set(blk.GetID(), NewValidityWindowBlock(blk))
				parentBlk = blk
			}

			// create the block so that we can test it against Execute directly.
			newBlk := Block{
				ParentID:  parentBlk.GetID(),
				Height:    uint64(testCase.timestamp),
				Timestamp: testCase.timestamp,
				blkID:     ids.GenerateTestID(),
			}

			for _, chunkExpiry := range testCase.chunks {
				_, chunkCert, err := node.BuildChunk(
					context.Background(),
					[]tx{
						{
							ID:     ids.Empty.Prefix(uint64(chunkExpiry)),
							Expiry: int64(chunkExpiry),
						},
					},
					int64(chunkExpiry),
					codec.Address{},
				)
				r.NoError(err)
				newBlk.ChunkCerts = append(newBlk.ChunkCerts, &chunkCert)
			}
			builtBlk, err := node.BuildBlock(context.Background(), parentBlk, testCase.timestamp)
			r.ErrorIs(err, testCase.buildWantErr)
			if err == nil {
				r.Equal(newBlk.ParentID, builtBlk.ParentID)
				r.Equal(newBlk.Height, builtBlk.Height)
				r.Equal(newBlk.Timestamp, builtBlk.Timestamp)
			}

			r.ErrorIs(node.Verify(context.Background(), parentBlk, newBlk), testCase.verifyWantErr)
		})
	}
}

// Nodes should request chunks referenced in accepted blocks
func TestAccept_RequestReferencedChunks(t *testing.T) {
	r := require.New(t)

	nodes, _ := newNodes(t, 2)
	node1 := nodes[0]
	node2 := nodes[1]

	chunk, _, err := node1.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	)
	r.NoError(err)
	blk, err := node1.BuildBlock(context.Background(), node1.LastAccepted, node1.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node1.Verify(context.Background(), node1.LastAccepted, blk))
	r.NoError(node1.Accept(context.Background(), blk))
	r.NoError(node2.Verify(context.Background(), node2.LastAccepted, blk))
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

	node := newTestNode(t)
	_, _, err := node.BuildChunk(
		context.Background(),
		[]tx{{ID: ids.GenerateTestID(), Expiry: 1}},
		100,
		codec.Address{123},
	)
	r.NoError(err)

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
}

func Test_Verify_BadBlock(t *testing.T) {
	tests := []struct {
		name    string
		blk     func(chunkCert ChunkCertificate, parent Block) Block
		wantErr error
	}{
		{
			name: "invalid parent",
			blk: func(chunkCert ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   ids.GenerateTestID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockParent,
		},
		{
			name: "invalid height - before parent",
			blk: func(chunkCert ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   parent.GetID(),
					Height:     parent.Height - 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid height - same as parent",
			blk: func(chunkCert ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   parent.GetID(),
					Height:     parent.Height,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid height - too far into future",
			blk: func(chunkCert ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 2,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid timestamp - before parent",
			blk: func(chunkCert ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp - 1,
					ChunkCerts: []*ChunkCertificate{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "invalid timestamp - same as parent",
			blk: func(chunkCert ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp,
					ChunkCerts: []*ChunkCertificate{&chunkCert},
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "invalid timestamp - too far into future",
			blk: func(_ ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + time.Minute.Nanoseconds(),
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "nil chunk certs",
			blk: func(_ ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + 1,
				}
			},
			wantErr: ErrEmptyBlock,
		},
		{
			name: "empty chunk certs",
			blk: func(_ ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:   parent.GetID(),
					Height:     parent.Height + 1,
					Timestamp:  parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate{},
				}
			},
			wantErr: ErrEmptyBlock,
		},
		{
			name: "invalid signature",
			blk: func(_ ChunkCertificate, parent Block) Block {
				return Block{
					ParentID:  parent.GetID(),
					Height:    parent.Height + 1,
					Timestamp: parent.Timestamp + 1,
					ChunkCerts: []*ChunkCertificate{
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

			node := newTestNode(t)
			_, chunkCert, err := node.BuildChunk(
				context.Background(),
				[]tx{{ID: ids.GenerateTestID(), Expiry: 2}},
				100,
				codec.Address{123},
			)
			r.NoError(err)
			r.ErrorIs(node.Verify(
				context.Background(),
				node.LastAccepted,
				tt.blk(chunkCert, node.LastAccepted),
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

func newTestNode(t *testing.T) *Node[tx] {
	nodes, _ := newNodes(t, 1)
	return nodes[0]
}

func newNodes(t *testing.T, n int) ([]*Node[tx], *testValidityWindowChainIndex) {
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

	indexer := newTestValidityWindowChainIndex()

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
			trace.Noop,
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
			Block{
				ParentID:  ids.Empty,
				Height:    0,
				Timestamp: 0,
				blkID:     ids.Empty,
			},
			1,
			1,
			indexer,
			testingDefaultValidityWindowDuration,
		)
		require.NoError(t, err)

		result = append(result, node)
	}

	// create a valid parent block for tests to verify off of
	node := result[0]
	_, _, err := node.BuildChunk(
		context.Background(),
		[]tx{
			{
				ID:      ids.ID{},
				Expiry:  123,
				Sponsor: codec.Address{},
			},
		},
		123,
		codec.Address{},
	)
	require.NoError(t, err)
	indexer.set(node.LastAccepted.GetID(), validityWindowBlock{Block: node.LastAccepted})

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	require.NoError(t, err)

	require.NoError(t, node.Verify(context.Background(), node.LastAccepted, blk))
	require.NoError(t, node.Accept(context.Background(), blk))
	indexer.set(blk.GetID(), validityWindowBlock{Block: blk})

	for _, n := range result[1:] {
		require.NoError(t, n.Verify(context.Background(), n.LastAccepted, blk))
		require.NoError(t, n.Accept(context.Background(), blk))
		indexer.set(blk.GetID(), validityWindowBlock{Block: blk})
	}

	return result, indexer
}

type testValidityWindowChainIndex struct {
	blocks map[ids.ID]validitywindow.ExecutionBlock[*emapChunkCertificate]
}

func (t *testValidityWindowChainIndex) GetExecutionBlock(_ context.Context, blkID ids.ID) (validitywindow.ExecutionBlock[*emapChunkCertificate], error) {
	if blk, ok := t.blocks[blkID]; ok {
		return blk, nil
	}
	return nil, database.ErrNotFound
}

func (t *testValidityWindowChainIndex) set(blkID ids.ID, blk validitywindow.ExecutionBlock[*emapChunkCertificate]) {
	t.blocks[blkID] = blk
}

func newTestValidityWindowChainIndex() *testValidityWindowChainIndex {
	return &testValidityWindowChainIndex{
		blocks: make(map[ids.ID]validitywindow.ExecutionBlock[*emapChunkCertificate]),
	}
}
