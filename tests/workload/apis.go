// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/utils"
)

func Ping(ctx context.Context, require *require.Assertions, uris []string) {
	clients := utils.Map(rpc.NewJSONRPCClient, uris)

	utils.ForEach(func(client *rpc.JSONRPCClient) {
		ok, err := client.Ping(ctx)
		require.NoError(err)
		require.True(ok)
	}, clients)
}

func GetNetwork(ctx context.Context, require *require.Assertions, uris []string, expectedNetworkID uint32, expectedChainID ids.ID) {
	clients := utils.Map(rpc.NewJSONRPCClient, uris)

	utils.ForEach(func(client *rpc.JSONRPCClient) {
		networkID, _, chainID, err := client.Network(ctx)
		require.NoError(err)
		require.Equal(expectedNetworkID, networkID)
		require.Equal(expectedChainID, chainID)
	}, clients)
}
