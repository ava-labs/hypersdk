// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ Tx                                                                         = (*tx)(nil)
	_ Marshaler[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse] = (*getChunkSignatureMarshaler)(nil)
)

func TestGetChunk(t *testing.T) {
	type addChunkArgs struct {
		txs    []tx
		expiry time.Time
	}

	tests := []struct {
		name          string
		chunksToBuild []addChunkArgs
	}{
		{
			name:          "no chunks",
			chunksToBuild: nil,
		},
		{
			name: "1 chunk with 1 tx",
			chunksToBuild: []addChunkArgs{
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
			chunksToBuild: []addChunkArgs{
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
			chunksToBuild: []addChunkArgs{
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
			chunksToBuild: []addChunkArgs{
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

			beneficiary := codec.CreateAddress(123, ids.GenerateTestID())
			sk, err := bls.NewSecretKey()
			r.NoError(err)

			nodeID := ids.GenerateTestNodeID()
			chunkStorage, err := NewChunkStorage[tx](NoVerifier[tx]{}, memdb.New())
			r.NoError(err)
			node, err := New[tx](nodeID, sk, beneficiary, chunkStorage, nil)
			r.NoError(err)

			// Build some chunks
			wantChunks := make([]Chunk[tx], 0)
			for _, args := range tt.chunksToBuild {
				chunk, err := node.NewChunk(args.txs, args.expiry)
				r.NoError(err)

				wantChunks = append(wantChunks, chunk)
			}

			client := NewTypedClient[*dsmr.GetChunkRequest, *dsmr.GetChunkResponse](
				p2ptest.NewClient(
					t,
					context.Background(),
					node.GetChunkHandler,
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				getChunkMarshaler{},
			)

			wg := &sync.WaitGroup{}
			wg.Add(len(tt.chunksToBuild))

			// Node must serve GetChunk requests for chunks that it
			// built
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
				r.NotEmpty(chunk.Signer)
				r.NotEmpty(chunk.Signature)
				//TODO check signature + aggregate public key
			}
		})
	}
}

// Node should be willing to sign valid chunks
func TestGetChunkSignature(t *testing.T) {
	// Can't sign duplicate chunks or chunks with duplicate txs
	// Can't sign invalid chunks
	tests := []struct {
		name    string
		chunks  []Chunk[tx]
		chunkID ids.ID
		wantErr *common.AppError
	}{
		{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			sk, err := bls.NewSecretKey()
			r.NoError(err)

			chunkStorage, err := NewChunkStorage[tx](NoVerifier[tx]{}, memdb.New())
			r.NoError(err)

			for _, chunk := range tt.chunks {
				r.NoError(chunkStorage.AddLocalChunkWithCert(chunk, &ChunkCertificate{}))
			}

			node, err := New[tx](
				ids.EmptyNodeID,
				sk,
				codec.Address{},
				chunkStorage,
				nil,
			)
			r.NoError(err)

			client := NewTypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse](
				p2ptest.NewClient(
					t,
					context.Background(),
					node.GetChunkSignatureHandler,
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
				getChunkSignatureMarshaler{},
			)

			done := make(chan struct{})
			onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkSignatureResponse, err error) {
				defer close(done)
			}

			r.NoError(client.AppRequest(
				context.Background(),
				set.Of(ids.EmptyNodeID),
				&dsmr.GetChunkSignatureRequest{
					Chunk: &dsmr.Chunk{
						Producer:     nil,
						Expiry:       0,
						Beneficiary:  nil,
						Transactions: nil,
						Signer:       nil,
						Signature:    nil,
					},
				},
				onResponse,
			))

			<-done
		})
	}
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
