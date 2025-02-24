// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain/chaintest"
)

func TestIndexerClient(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	var (
		numExecutedBlocks = 4
		blockWindow       = 2
	)
	indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow)

	jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
	require.NoError(err)

	httpServer := httptest.NewServer(jsonHandler)
	t.Cleanup(func() {
		httpServer.Close()
	})

	client := NewClient(httpServer.URL)
	executedBlock, err := client.GetBlockByHeight(ctx, executedBlocks[numExecutedBlocks-1].Block.Hght, chaintest.NewEmptyParser())
	require.NoError(err)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)

	executedBlock, err = client.GetBlockByHeight(ctx, executedBlocks[0].Block.Hght, chaintest.NewEmptyParser())
	require.Contains(err.Error(), errBlockNotFound.Error())
	require.Nil(executedBlock)

	executedBlock, err = client.GetLatestBlock(ctx, chaintest.NewEmptyParser())
	require.NoError(err)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)

	executedBlock, err = client.GetBlock(ctx, executedBlocks[numExecutedBlocks-1].Block.GetID(), chaintest.NewEmptyParser())
	require.NoError(err)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)

	executedBlock, err = client.GetBlock(ctx, ids.Empty, chaintest.NewEmptyParser())
	require.Contains(err.Error(), errBlockNotFound.Error())
	require.Nil(executedBlock)
}
