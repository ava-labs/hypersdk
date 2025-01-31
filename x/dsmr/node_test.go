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
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
	"github.com/ava-labs/hypersdk/x/dsmr/dsmrtest"

	snowValidators "github.com/ava-labs/avalanchego/snow/validators"
)

const (
	networkID                            = uint32(123)
	testingDefaultValidityWindowDuration = 5 * time.Second
)

var (
	_ Tx                    = (*dsmrtest.Tx)(nil)
	_ Verifier[dsmrtest.Tx] = (*failVerifier)(nil)

	chainID         = ids.Empty
	testRuleFactory = ruleFactory{
		rules: rules{
			validityWindow: int64(testingDefaultValidityWindowDuration),
		},
	}

	errTestingInvalidValidityWindow = errors.New("time validity window testing error")
)

// Test that chunks can be built through Node.BuildChunk
func TestNode_BuildChunk(t *testing.T) {
	tests := []struct {
		name        string
		txs         []dsmrtest.Tx
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
			txs: []dsmrtest.Tx{
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
			txs: []dsmrtest.Tx{
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
			r.ErrorIs(node.BuildChunk(
				context.Background(),
				tt.txs,
				tt.expiry,
				tt.beneficiary,
			), tt.wantErr)
			if tt.wantErr != nil {
				return
			}

			blk, err := node.BuildBlock(context.Background(), node.LastAccepted, tt.expiry)
			r.NoError(err)
			r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
			executedBlk, err := node.Accept(context.Background(), blk)
			r.NoError(err)
			r.Len(executedBlk.Chunks, 1)
			chunk := executedBlk.Chunks[0]

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

	nodes := newTestNodes(t, 2)
	node := nodes[0]

	r.NoError(node.BuildChunk(
		context.Background(),
		[]dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	))

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
	executedBlk, err := node.Accept(context.Background(), blk)
	r.NoError(err)
	r.Len(executedBlk.Chunks, 1)

	chunk := executedBlk.Chunks[0]
	client := NewGetChunkClient[dsmrtest.Tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node.ID,
		node.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[dsmrtest.Tx], err error) {
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

// Tests that unknown chunks are not available over p2p
func TestNode_GetChunk_UnknownChunk(t *testing.T) {
	r := require.New(t)

	node := newTestNode(t)
	client := NewGetChunkClient[dsmrtest.Tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node.ID,
		node.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, _ Chunk[dsmrtest.Tx], err error) {
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

// Tests that a Node serves chunks it accepts
func TestNode_AcceptedChunksAvailableOverGetChunk(t *testing.T) {
	type availableChunk struct {
		txs         []dsmrtest.Tx
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
					txs: []dsmrtest.Tx{
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
					txs: []dsmrtest.Tx{
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
					txs: []dsmrtest.Tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 0,
						},
					},
					expiry:      123,
					beneficiary: codec.Address{0},
				},
				{
					txs: []dsmrtest.Tx{
						{
							ID:     ids.GenerateTestID(),
							Expiry: 1,
						},
					},
					expiry:      123,
					beneficiary: codec.Address{1},
				},
				{
					txs: []dsmrtest.Tx{
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
					txs: []dsmrtest.Tx{
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
					txs: []dsmrtest.Tx{
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
					txs: []dsmrtest.Tx{
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
			for _, args := range tt.availableChunks {
				r.NoError(node.BuildChunk(
					context.Background(),
					args.txs,
					args.expiry,
					args.beneficiary,
				))
			}

			blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
			r.NoError(err)
			r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
			executedBlk, err := node.Accept(context.Background(), blk)
			r.NoError(err)
			r.Len(executedBlk.Chunks, len(tt.availableChunks))

			client := NewGetChunkClient[dsmrtest.Tx](p2ptest.NewClient(
				t,
				context.Background(),
				ids.EmptyNodeID,
				p2p.NoOpHandler{},
				node.ID,
				node.GetChunkHandler,
			))

			// Node must serve GetChunk requests for chunks that it accepted
			for _, chunk := range executedBlk.Chunks {
				done := make(chan struct{})
				r.NoError(client.AppRequest(
					context.Background(),
					node.ID,
					&dsmr.GetChunkRequest{
						ChunkId: chunk.id[:],
						Expiry:  chunk.Expiry,
					},
					func(_ context.Context, _ ids.NodeID, response Chunk[dsmrtest.Tx], err error) {
						defer close(done)

						r.NoError(err)

						r.Equal(node.ID, response.Producer)
						r.Equal(chunk.Expiry, response.Expiry)
						r.Equal(chunk.Beneficiary, response.Beneficiary)
						r.ElementsMatch(chunk.Txs, response.Txs)

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
					},
				))
				<-done
			}
		})
	}
}

// Node should be willing to sign valid chunks
func TestNode_GetChunkSignature_SignValidChunk(t *testing.T) {
	nodeID := ids.GenerateTestNodeID()
	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	pk := bls.PublicFromSecretKey(sk)
	tests := []struct {
		name                      string
		verifier                  Verifier[dsmrtest.Tx]
		producerNode              ids.NodeID
		wantErr                   error
		nodeLastAcceptedTimestamp int64
		chunkExpiry               int64
	}{
		{
			name:                      "invalid chunk",
			verifier:                  failVerifier{},
			wantErr:                   ErrInvalidChunk,
			producerNode:              ids.GenerateTestNodeID(),
			nodeLastAcceptedTimestamp: 1,
			chunkExpiry:               123,
		},
		{
			name: "invalid chunk ( bad producer id )",
			verifier: NewChunkVerifier[dsmrtest.Tx](
				networkID,
				chainID,
				pChain{
					validators: []Validator{{
						NodeID:    nodeID,
						Weight:    1,
						PublicKey: pk,
					}},
				},
				1,
				1,
				testRuleFactory,
			),
			wantErr:                   ErrInvalidChunk,
			producerNode:              ids.GenerateTestNodeID(),
			nodeLastAcceptedTimestamp: 1,
			chunkExpiry:               123,
		},
		{
			name: "invalid chunk ( chunk timestamp too old )",
			verifier: NewChunkVerifier[dsmrtest.Tx](
				networkID,
				chainID,
				pChain{
					validators: []Validator{{
						NodeID:    nodeID,
						Weight:    1,
						PublicKey: pk,
					}},
				},
				1,
				1,
				testRuleFactory,
			),
			wantErr:                   ErrInvalidChunk,
			producerNode:              nodeID,
			nodeLastAcceptedTimestamp: 5000,
			chunkExpiry:               123,
		},
		{
			name: "invalid chunk ( chunk timestamp too into the future )",
			verifier: NewChunkVerifier[dsmrtest.Tx](
				networkID,
				chainID,
				pChain{
					validators: []Validator{{
						NodeID:    nodeID,
						Weight:    1,
						PublicKey: pk,
					}},
				},
				1,
				1,
				testRuleFactory,
			),
			wantErr:                   ErrInvalidChunk,
			producerNode:              nodeID,
			nodeLastAcceptedTimestamp: 1,
			chunkExpiry:               2 + int64(testingDefaultValidityWindowDuration),
		},
		{
			name:         "valid chunk",
			producerNode: nodeID,
			verifier: NewChunkVerifier[dsmrtest.Tx](
				networkID,
				chainID,
				pChain{
					validators: []Validator{{
						NodeID:    nodeID,
						Weight:    1,
						PublicKey: pk,
					}},
				},
				1,
				1,
				testRuleFactory,
			),
			nodeLastAcceptedTimestamp: 1,
			chunkExpiry:               123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			signer := warp.NewSigner(sk, networkID, chainID)

			validators := []Validator{
				{
					NodeID:    nodeID,
					Weight:    1,
					PublicKey: pk,
				},
			}

			chunkStorage, err := NewChunkStorage[dsmrtest.Tx](tt.verifier, memdb.New())
			r.NoError(err)

			node, err := New[dsmrtest.Tx](
				logging.NoLog{},
				nodeID,
				networkID,
				chainID,
				pk,
				signer,
				chunkStorage,
				p2p.NoOpHandler{},
				acp118.NewHandler(
					ChunkSignatureRequestVerifier[dsmrtest.Tx]{
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
					BlockHeader: BlockHeader{
						ParentID:  ids.GenerateTestID(),
						Height:    0,
						Timestamp: 0,
					},
					blkID: ids.GenerateTestID(),
				},
				1,
				1,
				&validitywindowtest.MockTimeValidityWindow[*emapChunkCertificate]{},
				testRuleFactory,
			)
			r.NoError(err)

			r.NoError(node.BuildChunk(context.Background(), []dsmrtest.Tx{
				{
					ID:      ids.GenerateTestID(),
					Expiry:  456,
					Sponsor: codec.Address{4, 5, 6},
				},
			}, tt.nodeLastAcceptedTimestamp, codec.Address{123}))

			blk, err := node.BuildBlock(context.Background(), node.LastAccepted, tt.nodeLastAcceptedTimestamp)
			r.NoError(err)
			_, err = node.Accept(context.Background(), blk)
			r.NoError(err)

			unsignedChunk := UnsignedChunk[dsmrtest.Tx]{
				Producer:    tt.producerNode,
				Beneficiary: codec.Address{123},
				Expiry:      tt.chunkExpiry,
				Txs: []dsmrtest.Tx{
					{
						ID:      ids.GenerateTestID(),
						Expiry:  456,
						Sponsor: codec.Address{4, 5, 6},
					},
				},
			}
			chunk, err := signChunk[dsmrtest.Tx](unsignedChunk, networkID, chainID, pk, signer)
			r.NoError(err)

			packer := wrappers.Packer{MaxSize: MaxMessageSize}
			r.NoError(codec.LinearCodec.MarshalInto(ChunkReference{
				ChunkID:  ids.GenerateTestID(),
				Producer: ids.GenerateTestNodeID(),
				Expiry:   123,
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

// Nodes must persist chunks that they sign from other nodes
func TestGetChunkSignature_PersistAttestedBlocks(t *testing.T) {
	r := require.New(t)

	nodes := newTestNodes(t, 2)
	node1 := nodes[0]
	node2 := nodes[1]

	r.NoError(node1.BuildChunk(
		context.Background(),
		[]dsmrtest.Tx{{ID: ids.Empty, Expiry: 123}},
		123,
		codec.Address{123},
	))

	// Keep trying to build a block until we hear about the newly generated
	// chunk cert
	var (
		blk Block
		err error
	)
	for {
		blk, err = node2.BuildBlock(context.Background(), node2.LastAccepted, node2.LastAccepted.Timestamp+1)
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	r.NoError(node2.Verify(context.Background(), node2.LastAccepted, blk))
	executedBlk, err := node2.Accept(context.Background(), blk)
	r.NoError(err)
	r.Len(executedBlk.Chunks, 1)
	chunk := executedBlk.Chunks[0]

	client := NewGetChunkClient[dsmrtest.Tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node2.ID,
		node2.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[dsmrtest.Tx], err error) {
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

func TestNode_BuildBlock_IncludesChunks(t *testing.T) {
	type chunk struct {
		txs         []dsmrtest.Tx
		expiry      int64
		beneficiary codec.Address
	}

	tests := []struct {
		name               string
		chunks             func(parent Block) []chunk
		timestamp          func(parent Block) int64
		wantErr            error
		timeValidityWindow TimeValidityWindow[*emapChunkCertificate]
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
						txs: []dsmrtest.Tx{
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
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1,
							},
						},
						expiry: parent.Timestamp + 1,
					},
					{
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 2,
							},
						},
						expiry: parent.Timestamp + 2,
					},
					{
						txs: []dsmrtest.Tx{
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
						txs: []dsmrtest.Tx{
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
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1_000,
							},
						},
						expiry: parent.Timestamp + 1_000,
					},
					{
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 2_000,
							},
						},
						expiry: parent.Timestamp + 2_000,
					},
					{
						txs: []dsmrtest.Tx{
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
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1_000,
							},
						},
						expiry: parent.Timestamp + 1_000,
					},
					{
						txs: []dsmrtest.Tx{
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
		{
			name: "validity window error",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1_000,
							},
						},
						expiry: parent.Timestamp + 1_000,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 100
			},
			wantErr: errTestingInvalidValidityWindow,
			timeValidityWindow: &validitywindowtest.MockTimeValidityWindow[*emapChunkCertificate]{
				OnIsRepeat: func(context.Context, validitywindow.ExecutionBlock[*emapChunkCertificate], []*emapChunkCertificate, int64) (set.Bits, error) {
					return set.NewBits(), errTestingInvalidValidityWindow
				},
			},
		},
		{
			name: "no available chunk certs ( all duplicates )",
			chunks: func(parent Block) []chunk {
				return []chunk{
					{
						txs: []dsmrtest.Tx{
							{
								ID:     ids.GenerateTestID(),
								Expiry: parent.Timestamp + 1_000,
							},
						},
						expiry: parent.Timestamp + 1_000,
					},
				}
			},
			timestamp: func(parent Block) int64 {
				return parent.Timestamp + 100
			},
			wantErr: ErrNoAvailableChunkCerts,
			timeValidityWindow: &validitywindowtest.MockTimeValidityWindow[*emapChunkCertificate]{
				OnIsRepeat: func(context.Context, validitywindow.ExecutionBlock[*emapChunkCertificate], []*emapChunkCertificate, int64) (set.Bits, error) {
					marker := set.NewBits()
					marker.Add(0)
					return marker, nil
				},
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

			wantChunks := make([]chunk, 0)
			for _, chunk := range chunks {
				r.NoError(node.BuildChunk(
					context.Background(),
					chunk.txs,
					chunk.expiry,
					chunk.beneficiary,
				))

				// Only expect chunks that have not expired
				if chunk.expiry < timestamp {
					continue
				}

				wantChunks = append(wantChunks, chunk)
			}
			if tt.timeValidityWindow != nil {
				node.validityWindow = tt.timeValidityWindow
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
		})
	}
}

// Nodes should request chunks referenced in accepted blocks
func TestAccept_RequestReferencedChunks(t *testing.T) {
	r := require.New(t)

	nodes := newTestNodes(t, 2)
	node1 := nodes[0]
	node2 := nodes[1]

	r.NoError(node1.BuildChunk(
		context.Background(),
		[]dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 123}},
		123,
		codec.Address{123},
	))
	blk, err := node1.BuildBlock(context.Background(), node1.LastAccepted, node1.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node1.Verify(context.Background(), node1.LastAccepted, blk))
	_, err = node1.Accept(context.Background(), blk)
	r.NoError(err)
	r.NoError(node2.Verify(context.Background(), node2.LastAccepted, blk))
	executedBlk, err := node2.Accept(context.Background(), blk)
	r.NoError(err)
	r.Len(executedBlk.Chunks, 1)
	chunk := executedBlk.Chunks[0]

	client := NewGetChunkClient[dsmrtest.Tx](p2ptest.NewClient(
		t,
		context.Background(),
		ids.EmptyNodeID,
		p2p.NoOpHandler{},
		node2.ID,
		node2.GetChunkHandler,
	))

	done := make(chan struct{})
	onResponse := func(_ context.Context, _ ids.NodeID, response Chunk[dsmrtest.Tx], err error) {
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
	r.NoError(node.BuildChunk(
		context.Background(),
		[]dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 1}},
		100,
		codec.Address{123},
	))

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	r.NoError(err)
	r.NoError(node.Verify(context.Background(), node.LastAccepted, blk))
}

func Test_Verify_BadBlock(t *testing.T) {
	tests := []struct {
		name           string
		blk            func(parent Block) Block
		wantErr        error
		validityWindow TimeValidityWindow[*emapChunkCertificate]
	}{
		{
			name: "invalid parent",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  ids.GenerateTestID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp + 1,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrInvalidBlockParent,
		},
		{
			name: "invalid height - before parent",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height - 1,
						Timestamp: parent.Timestamp + 1,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid height - same as parent",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height,
						Timestamp: parent.Timestamp + 1,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid height - too far into future",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 2,
						Timestamp: parent.Timestamp + 1,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrInvalidBlockHeight,
		},
		{
			name: "invalid timestamp - before parent",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp - 1,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "invalid timestamp - same as parent",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "invalid timestamp - too far into future",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp + time.Minute.Nanoseconds(),
					},
				}
			},
			wantErr: ErrInvalidBlockTimestamp,
		},
		{
			name: "nil chunk certs",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp + 1,
					},
				}
			},
			wantErr: ErrEmptyBlock,
		},
		{
			name: "empty chunk certs",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp + 1,
					},
					ChunkCerts: []*ChunkCertificate{},
				}
			},
			wantErr: ErrEmptyBlock,
		},
		{
			name: "invalid signature",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp + 1,
					},
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
		{
			name: "invalid validity window",
			blk: func(parent Block) Block {
				return Block{
					BlockHeader: BlockHeader{
						ParentID:  parent.GetID(),
						Height:    parent.Height + 1,
						Timestamp: parent.Timestamp + 1,
					},
					ChunkCerts: parent.ChunkCerts,
				}
			},
			wantErr: errTestingInvalidValidityWindow,
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*emapChunkCertificate]{
				OnVerifyExpiryReplayProtection: func(context.Context, validitywindow.ExecutionBlock[*emapChunkCertificate]) error {
					return errTestingInvalidValidityWindow
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			node := newTestNode(t)
			r.NoError(node.BuildChunk(
				context.Background(),
				[]dsmrtest.Tx{{ID: ids.GenerateTestID(), Expiry: 2}},
				100,
				codec.Address{123},
			))
			blk, err := node.BuildBlock(context.Background(), node.LastAccepted, 100)
			r.NoError(err)
			_, err = node.Accept(context.Background(), blk)
			r.NoError(err)

			// optionally replace the node's validity window implementation in order to test
			// the handling of potential error cases.
			if tt.validityWindow != nil {
				node.validityWindow = tt.validityWindow
			}
			r.ErrorIs(node.Verify(
				context.Background(),
				node.LastAccepted,
				tt.blk(node.LastAccepted),
			), tt.wantErr)
		})
	}
}

type failVerifier struct{}

func (failVerifier) SetMin(int64) {}

func (failVerifier) Verify(Chunk[dsmrtest.Tx]) error {
	return errors.New("fail")
}

func (failVerifier) VerifyCertificate(context.Context, *ChunkCertificate) error {
	return errors.New("fail")
}

type testNode struct {
	ChunkStorage                  *ChunkStorage[dsmrtest.Tx]
	GetChunkHandler               p2p.Handler
	ChunkSignatureRequestHandler  p2p.Handler
	ChunkCertificateGossipHandler p2p.Handler
	Sk                            *bls.SecretKey
}

func newTestNode(t *testing.T) *Node[dsmrtest.Tx] {
	return newTestNodes(t, 1)[0]
}

func newTestNodes(t *testing.T, n int) []*Node[dsmrtest.Tx] {
	nodes := make([]testNode, 0, n)
	validators := make([]Validator, n)
	secretKeys := make([]*bls.SecretKey, n)
	var err error
	for i := 0; i < n; i++ {
		secretKeys[i], err = bls.NewSecretKey()
		require.NoError(t, err)
		pk := bls.PublicFromSecretKey(secretKeys[i])
		validators[i] = Validator{
			NodeID:    ids.GenerateTestNodeID(),
			Weight:    1,
			PublicKey: pk,
		}
	}

	for i := 0; i < n; i++ {
		signer := warp.NewSigner(secretKeys[i], networkID, chainID)
		verifier := NewChunkVerifier[dsmrtest.Tx](
			networkID,
			chainID,
			pChain{validators: validators},
			1,
			1,
			testRuleFactory,
		)
		chunkStorage, err := NewChunkStorage[dsmrtest.Tx](verifier, memdb.New())
		require.NoError(t, err)

		getChunkHandler := &GetChunkHandler[dsmrtest.Tx]{
			storage: chunkStorage,
		}
		chunkSignatureRequestHandler := acp118.NewHandler(ChunkSignatureRequestVerifier[dsmrtest.Tx]{
			verifier: verifier,
			storage:  chunkStorage,
		}, signer)
		chunkCertificateGossipHandler := ChunkCertificateGossipHandler[dsmrtest.Tx]{
			storage: chunkStorage,
		}

		nodes = append(nodes, testNode{
			ChunkStorage:                  chunkStorage,
			GetChunkHandler:               getChunkHandler,
			ChunkSignatureRequestHandler:  chunkSignatureRequestHandler,
			ChunkCertificateGossipHandler: chunkCertificateGossipHandler,
			Sk:                            secretKeys[i],
		})
	}

	result := make([]*Node[dsmrtest.Tx], 0, n)
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

		node, err := New[dsmrtest.Tx](
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
			Block{},
			1,
			1,
			&validitywindowtest.MockTimeValidityWindow[*emapChunkCertificate]{},
			testRuleFactory,
		)
		require.NoError(t, err)

		result = append(result, node)
	}

	// create a valid parent block for tests to verify off of
	node := result[0]
	require.NoError(t, node.BuildChunk(
		context.Background(),
		[]dsmrtest.Tx{
			{
				ID:      ids.ID{},
				Expiry:  123,
				Sponsor: codec.Address{},
			},
		},
		123,
		codec.Address{},
	))

	blk, err := node.BuildBlock(context.Background(), node.LastAccepted, node.LastAccepted.Timestamp+1)
	require.NoError(t, err)

	require.NoError(t, node.Verify(context.Background(), node.LastAccepted, blk))
	_, err = node.Accept(context.Background(), blk)
	require.NoError(t, err)

	for _, n := range result[1:] {
		require.NoError(t, n.Verify(context.Background(), n.LastAccepted, blk))
		_, err = n.Accept(context.Background(), blk)
		require.NoError(t, err)
	}

	return result
}

type ruleFactory struct {
	rules rules
}

func (r ruleFactory) GetRules(int64) Rules { return r.rules }

type rules struct {
	validityWindow int64
}

func (r rules) GetValidityWindow() int64 { return r.validityWindow }
