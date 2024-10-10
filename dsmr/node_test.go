package dsmr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
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
	t.Skip()

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

func TestP2P(t *testing.T) {
	p2ptest.NewClient()

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

var _ Client[tx] = (*client)(nil)

type client struct{}

func (c client) GetChunk(context.Context, ids.NodeID, ids.ID) (Chunk[tx], error) {
	return Chunk[tx]{}, nil
}
