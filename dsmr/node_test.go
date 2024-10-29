// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"
)

var (
	_ Tx                                                                         = (*tx)(nil)
	_ Marshaler[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse] = (*getChunkSignatureParser)(nil)
)

func TestGetChunk(t *testing.T) {
	type addChunkArgs struct {
		txs    []tx
		expiry time.Time
	}

	tests := []struct {
		name         string
		addChunkArgs []addChunkArgs
	}{
		{
			name:         "no chunks",
			addChunkArgs: nil,
		},
		{
			name: "1 chunk with 1 tx",
			addChunkArgs: []addChunkArgs{
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
			addChunkArgs: []addChunkArgs{
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
			addChunkArgs: []addChunkArgs{
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
			addChunkArgs: []addChunkArgs{
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
			node, err := New[tx](nodeID, sk, beneficiary, nil)
			r.NoError(err)

			expiry := time.Now()
			wantChunks := make([]Chunk[tx], 0)
			for _, args := range tt.addChunkArgs {
				chunk, err := node.NewChunk(args.txs, args.expiry)
				r.NoError(err)

				wantChunks = append(wantChunks, chunk)
			}

			client := GetChunkClient[tx]{
				client: p2ptest.NewClient(
					t,
					context.Background(),
					node.GetChunkHandler,
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
			}

			wg := &sync.WaitGroup{}
			wg.Add(len(tt.addChunkArgs))

			// Node must serve GetChunk requests for chunks that it
			// built
			gotChunks := make(map[ids.ID]Chunk[tx], 0)
			for _, chunk := range wantChunks {
				r.NoError(client.GetChunk(
					context.Background(),
					ids.EmptyNodeID,
					chunk.id,
					expiry,
					func(ctx context.Context, c Chunk[tx], err error) {
						defer wg.Done()

						r.NoError(err)

						gotChunks[c.id] = c
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
				//TODO check signature + aggregate public key
			}
		})
	}
}

//// Node should be willing to sign valid chunks
//func TestGetChunkSignature(t *testing.T) {
//	tests := []struct {
//		name string
//	}{}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			t.Skip()
//
//			r := require.New(t)
//			sk, err := bls.NewSecretKey()
//			r.NoError(err)
//
//			node, err := New[tx](ids.EmptyNodeID, sk, codec.Address{}, nil)
//			r.NoError(err)
//
//			r.NoError(node.NewChunk(Chunk[tx]{}))
//
//			client := NewTypedClient[*dsmr.GetChunkSignatureRequest, *dsmr.GetChunkSignatureResponse](
//				p2ptest.NewClient(
//					t,
//					context.Background(),
//					node.GetChunkSignatureHandler,
//					ids.EmptyNodeID,
//					ids.EmptyNodeID,
//				),
//				getChunkSignatureParser{},
//			)
//
//			onResponse := func(ctx context.Context, nodeID ids.NodeID, response *dsmr.GetChunkSignatureResponse, err error) {
//
//			}
//
//			r.NoError(client.AppRequest(
//				context.Background(),
//				set.Set[ids.NodeID]{},
//				nil,
//				onResponse,
//			))
//		})
//	}
//}

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

type getChunkSignatureParser struct{}

func (g getChunkSignatureParser) MarshalRequest(t *dsmr.GetChunkSignatureRequest) ([]byte, error) {
	return proto.Marshal(t)
}

func (g getChunkSignatureParser) UnmarshalResponse(bytes []byte) (*dsmr.GetChunkSignatureResponse, error) {
	response := dsmr.GetChunkSignatureResponse{}
	if err := proto.Unmarshal(bytes, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
