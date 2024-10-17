// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
)

func TestBuildBlock(t *testing.T) {
	tests := []struct {
		name           string
		chunksPerBlock int
		chunks         [][]tx
		wantTxs        []tx
	}{
		// TODO test empty chunks?
		{
			name:           "block not built",
			chunksPerBlock: 1,
			chunks:         [][]tx{},
			wantTxs:        []tx{},
		},
		{
			name:           "block not built",
			chunksPerBlock: 2,
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
				},
			},
			wantTxs: []tx{
				{
					ID:     ids.ID{0},
					Expiry: 0,
				},
			},
		},
		{
			name:           "block built - 1 chunk with 1 tx",
			chunksPerBlock: 1,
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
				},
			},
			wantTxs: []tx{
				{
					ID:     ids.ID{0},
					Expiry: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			node, err := New[tx](nil)
			require.NoError(err)

			for _, chunk := range tt.chunks {
				require.NoError(node.BuildChunk(chunk))
			}

			blk, err := node.NewBlock()
			require.NoError(err)

			client := GetChunkClient[tx]{
				client: p2ptest.NewClient(
					t,
					context.Background(),
					node.GetChunkHandler,
					ids.EmptyNodeID,
					ids.EmptyNodeID,
				),
			}

			gotTxs := make([]tx, 0)
			for _, chunk := range blk.Chunks {
				require.NoError(client.GetChunk(
					context.Background(),
					ids.EmptyNodeID,
					chunk.ChunkID,
					func(ctx context.Context, c Chunk[tx], err error) {
						require.NoError(err)

						for _, tx := range c.Txs {
							gotTxs = append(gotTxs, tx)
						}
					},
				))
			}

			require.ElementsMatch(tt.wantTxs, gotTxs)
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
