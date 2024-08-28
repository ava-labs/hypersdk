// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"
	"encoding/json"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/codec"
)

func Ping(ctx context.Context, require *require.Assertions, uris []string) {
	for _, uri := range uris {
		client := jsonrpc.NewJSONRPCClient(uri)
		ok, err := client.Ping(ctx)
		require.NoError(err)
		require.True(ok)
	}
}

func GetNetwork(ctx context.Context, require *require.Assertions, uris []string, expectedNetworkID uint32, expectedChainID ids.ID) {
	for _, uri := range uris {
		client := jsonrpc.NewJSONRPCClient(uri)
		networkID, _, chainID, err := client.Network(ctx)
		require.NoError(err)
		require.Equal(expectedNetworkID, networkID)
		require.Equal(expectedChainID, chainID)
	}
}

func GetABI(ctx context.Context, require *require.Assertions, uris []string) {
	for _, uri := range uris {
		client := jsonrpc.NewJSONRPCClient(uri)
		abiString, err := client.GetABI(ctx)
		require.NoError(err)
		require.NotEmpty(abiString)

		var abi []codec.SingleActionABI
		err = json.Unmarshal([]byte(abiString), &abi)
		require.NoError(err)

		require.NotEmpty(abi)
		require.NotEmpty(abi[0].Name)
	}
}
