// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/rpc"
)

func Ping(ctx context.Context, require *require.Assertions, uris []string) {
	for _, uri := range uris {
		client := rpc.NewJSONRPCClient(uri)
		ok, err := client.Ping(ctx)
		require.NoError(err)
		require.True(ok)
	}
}

func GetNetwork(ctx context.Context, require *require.Assertions, uris []string, expectedNetworkID uint32, expectedChainID ids.ID) {
	for _, uri := range uris {
		client := rpc.NewJSONRPCClient(uri)
		networkID, _, chainID, err := client.Network(ctx)
		require.NoError(err)
		require.Equal(expectedNetworkID, networkID)
		require.Equal(expectedChainID, chainID)
	}
}
