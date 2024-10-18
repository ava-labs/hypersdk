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
)

func TestGetChunk(t *testing.T) {
	tests := []struct {
		name   string
		chunks [][]tx
	}{
		// TODO test empty chunks?
		{
			name:   "block not built",
			chunks: [][]tx{},
		},
		{
			name: "block built - 1 chunk with 1 tx",
			chunks: [][]tx{
				{
					{
						ID:     ids.ID{0},
						Expiry: 0,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			node, err := New[tx](nil)
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

			//TODO check chunks instead of just txs?
			gotTxs := make([]tx, 0)
			for _, chunk := range wantChunks {
				require.NoError(client.GetChunk(
					context.Background(),
					ids.EmptyNodeID,
					chunk.id,
					expiry,
					func(ctx context.Context, c Chunk[tx], err error) {
						defer wg.Done()

						require.NoError(err)

						gotTxs = append(gotTxs, c.Txs...)
					},
				))
			}

			wg.Wait()

			wantTxs := make([]tx, 0)
			for _, txs := range tt.chunks {
				wantTxs = append(wantTxs, txs...)
			}

			require.ElementsMatch(wantTxs, gotTxs)
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
