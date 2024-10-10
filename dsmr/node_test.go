package dsmr

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/utils/set"
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

	require := require.New(T)

	blks := make(chan Block)

	node := New[tx](client{}, 3)
	go func() {
		_ = node.Run(blks)
	}()

	require.NoError(node.AddTx(tx{}))
	require.NoError(node.AddTx(tx{}))
	require.NoError(node.AddTx(tx{}))

	<-blks
}

func TestP2P(t *testing.T) {
	require := require.New(t)

	node := New[tx](client{}, 1)
	client := p2ptest.NewClient(t, context.Background(), node, ids.EmptyNodeID, ids.EmptyNodeID)

	onResponse := func(ctx context.Context, nodeID ids.NodeID, appResponseBytes []byte, err error) {

	}

	require.NoError(client.AppRequest(
		context.Background(),
		set.Of(ids.EmptyNodeID),
		[]byte("request"),
		onResponse,
	))
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
