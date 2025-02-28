// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

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
	indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, 3)

	jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
	require.NoError(err)

	httpServer := httptest.NewServer(jsonHandler)
	t.Cleanup(func() {
		httpServer.Close()
	})

	parser := chaintest.NewTestParser(require)

	client := NewClient(httpServer.URL)
	executedBlock, err := client.GetBlockByHeight(ctx, executedBlocks[numExecutedBlocks-1].Block.Hght, parser)
	require.NoError(err)
	// initStateKeys(executedBlock.Block)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)

	executedBlock, err = client.GetBlockByHeight(ctx, executedBlocks[0].Block.Hght, parser)
	require.Contains(err.Error(), errBlockNotFound.Error())
	require.Nil(executedBlock)

	executedBlock, err = client.GetLatestBlock(ctx, parser)
	require.NoError(err)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)

	executedBlock, err = client.GetBlock(ctx, executedBlocks[numExecutedBlocks-1].Block.GetID(), parser)
	require.NoError(err)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)

	executedBlock, err = client.GetBlock(ctx, ids.Empty, parser)
	require.Contains(err.Error(), errBlockNotFound.Error())
	require.Nil(executedBlock)

	// test for missing transaction
	txResponse, found, err := client.GetTxResults(ctx, ids.GenerateTestID())
	require.False(found)
	require.Equal(GetTxResponse{}, txResponse)
	require.NoError(err)

	txResponse, tx, found, err := client.GetTx(ctx, ids.GenerateTestID(), parser)
	require.False(found)
	require.Equal(GetTxResponse{}, txResponse)
	require.Nil(tx)
	require.NoError(err)

	timeoutCtx, ctxCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer ctxCancel()
	success, fee, err := client.WaitForTransaction(timeoutCtx, 1*time.Millisecond, ids.GenerateTestID())
	require.False(success)
	require.Zero(fee)
	require.ErrorIs(err, context.DeadlineExceeded)

	// request one of the transactions included in the latest block.
	txResponse, tx, found, err = client.GetTx(ctx, executedBlocks[numExecutedBlocks-1].Block.Txs[0].GetID(), parser)
	require.True(found)
	require.Equal(GetTxResponse{
		TxBytes:   executedBlocks[numExecutedBlocks-1].Block.Txs[0].Bytes(),
		Timestamp: executedBlocks[numExecutedBlocks-1].Block.Tmstmp,
		Result:    executedBlocks[numExecutedBlocks-1].Results[0],
	}, txResponse)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block.Txs[0], tx)
	require.NoError(err)
}
