// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type Client[T Tx] interface {
	GetChunk(ctx context.Context, nodeID ids.NodeID, chunkID ids.ID) (Chunk[T], error)
}
