// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain/chaintest"
)

type shutdownFunc func()

func createTestServer(t *testing.T, indexer *Indexer) (string, shutdownFunc) {
	require := require.New(t)

	server := &Server{
		tracer:  trace.Noop,
		indexer: indexer,
	}

	jsonHandler, err := api.NewJSONRPCHandler(Name, server)
	require.NoError(err)

	// Listen on a random available port.
	listener, err := net.Listen("tcp", "localhost:0") // ":0" tells the OS to choose a port.
	require.NoError(err)

	uri := "http://" + listener.Addr().String()

	httpServer := http.Server{
		Handler:           jsonHandler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	// Start the server using the listener.  This blocks until the server shuts down.
	go func() {
		require.Equal(http.ErrServerClosed, httpServer.Serve(listener))
	}()

	return uri, func() {
		require.NoError(httpServer.Shutdown(context.Background()))
	}
}

func TestIndexerClient(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	var (
		numExecutedBlocks = 4
		blockWindow       = 2
	)
	indexer, executedBlocks, _ := createTestIndexer(t, ctx, numExecutedBlocks, blockWindow)

	uri, serverShutdown := createTestServer(t, indexer)
	defer serverShutdown()

	client := NewClient(uri, chaintest.NewEmptyParser())
	executedBlock, err := client.GetBlockByHeight(context.Background(), executedBlocks[numExecutedBlocks-1].Block.Hght)
	require.NoError(err)
	require.NotNil(executedBlock)

	executedBlock, err = client.GetBlockByHeight(context.Background(), executedBlocks[0].Block.Hght)
	require.Contains(err.Error(), errBlockNotFound.Error())
	require.Nil(executedBlock)

	executedBlock, err = client.GetLatestBlock(context.Background())
	require.NoError(err)
	require.Equal(executedBlocks[numExecutedBlocks-1].Block.Hght, executedBlock.Block.Hght)

	executedBlock, err = client.GetBlock(context.Background(), executedBlocks[numExecutedBlocks-1].Block.GetID())
	require.NoError(err)
	require.NotNil(executedBlock)

	executedBlock, err = client.GetBlock(context.Background(), ids.Empty)
	require.Contains(err.Error(), errBlockNotFound.Error())
	require.Nil(executedBlock)
}
