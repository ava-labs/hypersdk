// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
)

func TestIndexerClientBlocks(t *testing.T) {
	const (
		blockWindow = 2
		numTxs      = 3
	)

	testCases := []struct {
		name              string
		blkHeight         uint64
		blkHeightErr      error
		numExecutedBlocks int
	}{
		{
			name:              "success",
			blkHeight:         3,
			blkHeightErr:      nil,
			numExecutedBlocks: 4,
		},
		{
			name:              "missing block",
			blkHeight:         0,
			blkHeightErr:      errBlockNotFound,
			numExecutedBlocks: 4,
		},
		{
			name:              "no blocks",
			blkHeight:         0,
			blkHeightErr:      errBlockNotFound,
			numExecutedBlocks: 0,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()
			indexer, executedBlocks, _ := createTestIndexer(t, ctx, tt.numExecutedBlocks, blockWindow, numTxs)

			jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
			r.NoError(err)

			httpServer := httptest.NewServer(jsonHandler)
			t.Cleanup(func() {
				httpServer.Close()
			})

			parser := chaintest.NewTestParser()

			client := NewClient(httpServer.URL)

			executedBlock, err := client.GetBlockByHeight(ctx, tt.blkHeight, parser)
			if tt.blkHeightErr == nil {
				r.NoError(err)
				expectedBlock := executedBlocks[tt.blkHeight-1]
				r.Equal(expectedBlock.Block, executedBlock.Block)
			} else {
				r.ErrorContains(err, tt.blkHeightErr.Error())
			}

			expectedBlkID := ids.Empty
			if tt.blkHeight > 0 {
				expectedBlock := executedBlocks[tt.blkHeight-1]
				expectedBlkID = expectedBlock.Block.GetID()
			}

			executedBlock, err = client.GetBlock(ctx, expectedBlkID, parser)
			if tt.blkHeightErr == nil {
				r.NoError(err)
				r.Equal(executedBlocks[tt.blkHeight-1].Block, executedBlock.Block)
			} else {
				r.ErrorContains(err, tt.blkHeightErr.Error())
			}

			executedBlock, err = client.GetLatestBlock(ctx, parser)
			if tt.numExecutedBlocks == 0 {
				r.ErrorContains(err, database.ErrNotFound.Error())
			} else {
				r.NoError(err)
				r.Equal(executedBlocks[tt.numExecutedBlocks-1].Block, executedBlock.Block)
			}
		})
	}
}

func TestIndexerClientTransactions(t *testing.T) {
	const (
		numExecutedBlocks = 4
		blockWindow       = 2
		numTxs            = 3
	)

	parser := chaintest.NewTestParser()
	badParser := &chain.TxTypeParser{
		ActionRegistry: codec.NewTypeParser[chain.Action](),
		AuthRegistry:   codec.NewTypeParser[chain.Auth](),
	}

	testCases := []struct {
		name            string
		blockIndex      int
		txIndex         int
		getTxResultsErr error
		getTxErr        error
		found           bool
		parser          *chain.TxTypeParser
		malformedBlock  bool
	}{
		{
			name:            "success",
			blockIndex:      numExecutedBlocks - 1,
			txIndex:         0,
			found:           true,
			getTxResultsErr: nil,
			getTxErr:        nil,
			parser:          parser,
			malformedBlock:  false,
		},
		{
			name:            "missing transaction",
			blockIndex:      0,
			txIndex:         0,
			found:           false,
			getTxResultsErr: nil,
			getTxErr:        nil,
			parser:          parser,
			malformedBlock:  false,
		},
		{
			name:            "badParser",
			blockIndex:      numExecutedBlocks - 1,
			txIndex:         numTxs - 1,
			found:           true,
			getTxResultsErr: nil,
			getTxErr:        errTxUnmarshalingFailed,
			parser:          badParser,
			malformedBlock:  false,
		},
		{
			name:            "malformed block",
			blockIndex:      numExecutedBlocks - 1,
			txIndex:         0,
			found:           false,
			getTxResultsErr: errTxResultNotFound,
			getTxErr:        errTxResultNotFound,
			parser:          parser,
			malformedBlock:  true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()
			indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)

			jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
			r.NoError(err)

			httpServer := httptest.NewServer(jsonHandler)
			t.Cleanup(func() {
				httpServer.Close()
			})

			client := NewClient(httpServer.URL)

			executedBlock := executedBlocks[tt.blockIndex]
			executedTx := executedBlock.Block.Txs[tt.txIndex]

			if tt.malformedBlock {
				// create a malformed block and have the indexer add it.
				// clearing out the results ensure that we won't be able to find the result corresponding to any
				// of the transactions.
				executedBlock.ExecutionResults.Results = []*chain.Result{}
				r.NoError(indexer.Notify(ctx, executedBlock))
			}

			txResponse, found, err := client.GetTxResults(ctx, executedTx.GetID())
			if tt.getTxResultsErr != nil {
				r.Error(err)
				r.ErrorContains(err, tt.getTxResultsErr.Error())
				err = nil
			}
			r.NoError(err)
			r.Equal(tt.found, found)
			if tt.found {
				r.Equal(GetTxResponse{
					TxBytes:   executedTx.Bytes(),
					Timestamp: executedBlock.Block.Tmstmp,
					Result:    executedBlock.ExecutionResults.Results[tt.txIndex],
				}, txResponse)
			}

			txResponse, tx, found, err := client.GetTx(ctx, executedTx.GetID(), tt.parser)
			if tt.getTxErr != nil {
				r.Error(err)
				r.ErrorContains(err, tt.getTxErr.Error())
				return
			}
			r.NoError(err)
			r.Equal(tt.found, found)
			if !tt.found {
				return
			}

			r.Equal(GetTxResponse{
				TxBytes:   executedTx.Bytes(),
				Timestamp: executedBlock.Block.Tmstmp,
				Result:    executedBlock.ExecutionResults.Results[tt.txIndex],
			}, txResponse)
			r.Equal(executedTx, tx)
		})
	}
}

func TestIndexerClientWaitForTransaction(t *testing.T) {
	const (
		numExecutedBlocks = 2
		blockWindow       = 1
		numTxs            = 3
	)
	r := require.New(t)
	ctx := context.Background()
	indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow, numTxs)

	jsonHandler, err := api.NewJSONRPCHandler(Name, NewServer(trace.Noop, indexer))
	r.NoError(err)

	httpServer := httptest.NewServer(jsonHandler)
	t.Cleanup(func() {
		httpServer.Close()
	})

	client := NewClient(httpServer.URL)

	timeoutCtx, ctxCancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer ctxCancel()
	_, _, err = client.WaitForTransaction(timeoutCtx, 1*time.Millisecond, ids.GenerateTestID())
	r.ErrorIs(err, context.DeadlineExceeded)

	// wait for a past transaction.
	lastExecutedBlock := executedBlocks[numExecutedBlocks-1]
	lastTx := lastExecutedBlock.Block.Txs[numTxs-1]
	lastTxResult := lastExecutedBlock.ExecutionResults.Results[numTxs-1]
	success, fee, err := client.WaitForTransaction(ctx, 1*time.Millisecond, lastTx.GetID())
	r.NoError(err)
	r.Equal(lastTxResult.Success, success)
	r.Equal(lastTxResult.Fee, fee)
}
