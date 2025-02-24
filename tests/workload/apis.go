// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
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

func GetABI(ctx context.Context, require *require.Assertions, uris []string, expectedABI abi.ABI) {
	for _, uri := range uris {
		client := jsonrpc.NewJSONRPCClient(uri)
		actualABI, err := client.GetABI(ctx)
		require.NoError(err)

		require.GreaterOrEqual(len(actualABI.Actions), 1)
		require.NotEmpty(actualABI.Actions[0].Name)
		require.Equal(expectedABI, actualABI)
	}
}

func GetBlocks(ctx context.Context, require *require.Assertions, parser chain.Parser, uris []string) {
	for _, uri := range uris {
		client := indexer.NewClient(uri)
		latestBlock, err := client.GetLatestBlock(ctx, parser)
		require.NoError(err)
		blockByHeight, err := client.GetBlockByHeight(ctx, latestBlock.Block.Hght, parser)
		require.NoError(err)
		require.Equal(latestBlock, blockByHeight)
		blockByID, err := client.GetBlock(ctx, latestBlock.Block.GetID(), parser)
		require.NoError(err)
		require.Equal(latestBlock, blockByID)
	}
}
