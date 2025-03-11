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

func TestIndexerClientBlocks(t *testing.T) {
	const (
		numExecutedBlocks = 4
		blockWindow       = 2
		numTxs            = 3
	)

	testCases := []struct {
		name      string
		blkHeight uint64
		err       error
	}{
		{
			name:      "success",
			blkHeight: numExecutedBlocks - 1,
			err:       nil,
		},
		{
			name:      "missing block",
			blkHeight: 0,
			err:       errBlockNotFound,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctx := context.Background()
			indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)

			jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
			require.NoError(err)

			httpServer := httptest.NewServer(jsonHandler)
			t.Cleanup(func() {
				httpServer.Close()
			})

			parser := chaintest.NewTestParser()

			client := NewClient(httpServer.URL)
			executedBlock, err := client.GetBlockByHeight(ctx, executedBlocks[tt.blkHeight].Block.Hght, parser)
			if tt.err == nil {
				require.NoError(err)
				require.Equal(executedBlocks[tt.blkHeight].Block, executedBlock.Block)
			} else {
				require.Contains(err.Error(), tt.err.Error())
			}

			executedBlock, err = client.GetBlock(ctx, executedBlocks[tt.blkHeight].Block.GetID(), parser)
			if tt.err == nil {
				require.NoError(err)
				require.Equal(executedBlocks[tt.blkHeight].Block, executedBlock.Block)
			} else {
				require.Contains(err.Error(), tt.err.Error())
			}

			executedBlock, err = client.GetLatestBlock(ctx, parser)
			require.NoError(err)
			require.Equal(executedBlocks[numExecutedBlocks-1].Block, executedBlock.Block)
		})
	}
}

func TestIndexerClientTransactions(t *testing.T) {
	const (
		numExecutedBlocks = 4
		blockWindow       = 2
		numTxs            = 3
	)

	testCases := []struct {
		name     string
		txBlkIdx int
		txIdx    int
		err      error
		found    bool
	}{
		{
			name:     "success",
			txBlkIdx: numExecutedBlocks - 1,
			txIdx:    0,
			found:    true,
			err:      nil,
		},
		{
			name:     "missing transaction",
			txBlkIdx: 0,
			txIdx:    0,
			found:    false,
			err:      nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctx := context.Background()
			indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)

			jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
			require.NoError(err)

			httpServer := httptest.NewServer(jsonHandler)
			t.Cleanup(func() {
				httpServer.Close()
			})

			parser := chaintest.NewTestParser()

			client := NewClient(httpServer.URL)

			txResponse, found, err := client.GetTxResults(ctx, executedBlocks[tt.txBlkIdx].Block.Txs[tt.txIdx].GetID())
			require.Equal(tt.err, err)
			require.Equal(tt.found, found)
			if tt.found {
				require.Equal(GetTxResponse{
					TxBytes:   executedBlocks[tt.txBlkIdx].Block.Txs[tt.txIdx].Bytes(),
					Timestamp: executedBlocks[tt.txBlkIdx].Block.Tmstmp,
					Result:    executedBlocks[tt.txBlkIdx].ExecutionResults.Results[tt.txIdx],
				}, txResponse)
			}

			txResponse, tx, found, err := client.GetTx(ctx, executedBlocks[tt.txBlkIdx].Block.Txs[tt.txIdx].GetID(), parser)
			require.Equal(tt.err, err)
			require.Equal(tt.found, found)
			if tt.found {
				require.Equal(GetTxResponse{
					TxBytes:   executedBlocks[tt.txBlkIdx].Block.Txs[tt.txIdx].Bytes(),
					Timestamp: executedBlocks[tt.txBlkIdx].Block.Tmstmp,
					Result:    executedBlocks[tt.txBlkIdx].ExecutionResults.Results[tt.txIdx],
				}, txResponse)
				require.Equal(executedBlocks[tt.txBlkIdx].Block.Txs[tt.txIdx], tx)
			}
		})
	}
}

func TestIndexerClientWaitForTransaction(t *testing.T) {
	const (
		numExecutedBlocks = 4
		blockWindow       = 2
		numTxs            = 3
	)
	require := require.New(t)
	ctx := context.Background()
	indexer, _, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)

	jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
	require.NoError(err)

	httpServer := httptest.NewServer(jsonHandler)
	t.Cleanup(func() {
		httpServer.Close()
	})

	client := NewClient(httpServer.URL)

	timeoutCtx, ctxCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer ctxCancel()
	_, _, err = client.WaitForTransaction(timeoutCtx, 1*time.Millisecond, ids.GenerateTestID())
	require.ErrorIs(err, context.DeadlineExceeded)
}
