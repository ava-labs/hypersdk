// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/hypersdk/codec"
)

func TestGetChunk(t *testing.T) {
	tests := []struct {
		name   string
		chunks [][]tx
	}{
		{
			name:   "no chunks",
			chunks: [][]tx{},
		},
		{
			name: "1 chunk with 1 tx",
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
				},
			},
		},
		{
			name: "1 chunk with 3 txs",
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
					{
						ID:     ids.ID{1},
						Expiry: 1,
					},
					{
						ID:     ids.ID{2},
						Expiry: 2,
					},
				},
			},
		},
		{
			name: "3 chunks with 1 tx",
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
				},
				{
					{
						ID:     ids.ID{1},
						Expiry: 1,
					},
				},
			},
		},
		{
			name: "3 chunks with 3 txs",
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
					{
						ID:     ids.ID{1},
						Expiry: 1,
					},
					{
						ID:     ids.ID{2},
						Expiry: 2,
					},
				},
				{
					{
						ID:     ids.ID{3},
						Expiry: 3,
					},
					{
						ID:     ids.ID{4},
						Expiry: 4,
					},
					{
						ID:     ids.ID{5},
						Expiry: 5,
					},
				},
				{
					{
						ID:     ids.ID{6},
						Expiry: 6,
					},
					{
						ID:     ids.ID{7},
						Expiry: 7,
					},
					{
						ID:     ids.ID{8},
						Expiry: 8,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			nodeID := ids.GenerateTestNodeID()
			beneficiary := codec.CreateAddress(123, ids.GenerateTestID())
			sk, err := bls.NewSecretKey()
			require.NoError(err)

			node, err := New[tx](nodeID, sk, beneficiary, nil)
			require.NoError(err)

			expiry := time.Now()
			wantChunks := make([]Chunk[tx], 0)
			for _, txs := range tt.chunks {
				chunk, err := node.BuildChunk(txs, expiry)
				require.NoError(err)

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
			wg.Add(len(wantChunks))

			gotChunks := make(map[ids.ID]Chunk[tx], 0)
			for _, chunk := range wantChunks {
				require.NoError(client.GetChunk(
					context.Background(),
					ids.EmptyNodeID,
					chunk.id,
					expiry,
					func(ctx context.Context, c Chunk[tx], err error) {
						defer wg.Done()

						require.NoError(err)

						gotChunks[c.id] = c
					},
				))
			}

			wg.Wait()

			for _, chunk := range wantChunks {
				require.Contains(gotChunks, chunk.id)

				gotChunk := gotChunks[chunk.id]
				require.Equal(nodeID, gotChunk.Producer)
				require.Equal(chunk.Expiry, gotChunk.Expiry)
				require.Equal(beneficiary, gotChunk.Beneficiary)
				require.ElementsMatch(chunk.Txs, gotChunk.Txs)
				//TODO check signature + aggregate public key
			}
		})
	}
}

var _ Tx = (*tx)(nil)

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
