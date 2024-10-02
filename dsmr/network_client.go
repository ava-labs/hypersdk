package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type Client[T Tx] interface {
	GetChunk(ctx context.Context, nodeID ids.NodeID, chunkID ids.ID) (Chunk[T], error)
}
