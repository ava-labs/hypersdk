package dsmr

import (
	"context"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

// TODO
// TODO block building heuristics other than # tx/chunks like time
// (txsPerChunk=1, chunksPerBlock=1, peer respond) Read tx -> block built
// (txsPerChunk=1, chunksPerBlock=1, peer no respond) Read tx -> block not built

// (txsPerChunk=2, chunksPerBlock=1, peer respond) Read tx -> block not built
// (txsPerChunk=2, chunksPerBlock=1, peer no respond) Read tx -> block not built
//
// (txsPerChunk=1, chunksPerBlock=2, peer respond) Read tx -> block not built
// (txsPerChunk=1, chunksPerBlock=2, peer no respond) Read tx -> block not built
func TestNode(t *testing.T) {
	n := New[tx](client{}, 3)
	blks := make(chan Block)

	go func() {
		_ = n.Run(blks)
	}()

	go func() {
		require.NoError(t, n.AddTx(tx{}))
		require.NoError(t, n.AddTx(tx{}))
		require.NoError(t, n.AddTx(tx{}))
	}()

	<-blks
}

var _ Tx = (*tx)(nil)

type tx struct{}

func (t tx) GetID() ids.ID {
	return ids.Empty
}

func (t tx) GetExpiry() time.Time {
	return time.Time{}
}

var _ Client[tx] = (*client)(nil)

type client struct{}

func (c client) GetChunk(context.Context, ids.NodeID, ids.ID) (Chunk[tx], error) {
	return Chunk[tx]{}, nil
}
