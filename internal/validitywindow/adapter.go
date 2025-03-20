// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/typedclient"
)

var _ NetworkBlockFetcher = (*P2PBlockFetcher)(nil)

// P2PBlockFetcher is a concrete implementation of NetworkBlockFetcher using the p2p network client
type P2PBlockFetcher struct {
	typedClient *typedclient.SyncTypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte]
}

func NewP2PBlockFetcher(p2pClient typedclient.P2PClient) *P2PBlockFetcher {
	return &P2PBlockFetcher{
		typedClient: typedclient.NewSyncTypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte](p2pClient, &blockFetcherMarshaler{}),
	}
}

// FetchBlocksFromPeer sends a block fetch request to the specified peer and returns the response
func (f *P2PBlockFetcher) FetchBlocksFromPeer(ctx context.Context, nodeID ids.NodeID, request *BlockFetchRequest) (*BlockFetchResponse, error) {
	return f.typedClient.SyncAppRequest(ctx, nodeID, request)
}
