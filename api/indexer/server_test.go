// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/fees"
)

func TestEncodingValidate(t *testing.T) {
	tests := []struct {
		input  Encoding
		exp    Encoding
		expErr error
	}{
		{
			input: JSON,
			exp:   JSON,
		},
		{
			input: Hex,
			exp:   Hex,
		},
		{
			input: Encoding(""),
			exp:   JSON,
		},
		{
			input:  Encoding("another"),
			expErr: ErrInvalidEncodingParameter,
		},
	}

	for _, test := range tests {
		err := test.input.Validate()
		if test.expErr != nil {
			require.EqualError(t, ErrInvalidEncodingParameter, err.Error())
		} else {
			require.Equal(t, test.exp, test.input)
		}
	}
}

func TestBlockRequests(t *testing.T) {
	require := require.New(t)
	blocks := chaintest.GenerateEmptyExecutedBlocks(require, ids.GenerateTestID(), 0, 0, 0, 1)
	server, client := newIndexerHTTPServerAndClient(require, blocks[0])
	defer server.Close()

	res, err := client.GetBlock(context.Background(), blocks[0].BlockID, chaintest.NewEmptyParser())
	require.NoError(err)
	require.Equal(blocks[0].BlockID, res.BlockID)

	res, err = client.GetBlockByHeight(context.Background(), blocks[0].Block.Hght, chaintest.NewEmptyParser())
	require.NoError(err)
	require.Equal(blocks[0].BlockID, res.BlockID)

	res, err = client.GetLatestBlock(context.Background(), chaintest.NewEmptyParser())
	require.NoError(err)
	require.Equal(blocks[0].BlockID, res.BlockID)
}

func newIndexerHTTPServerAndClient(require *require.Assertions, block *chain.ExecutedBlock) (*httptest.Server, *Client) {
	rpcServer := &Server{
		tracer: trace.Noop,
		indexer: &mockIndexer{
			execuredBlock: block,
		},
	}
	handler, err := api.NewJSONRPCHandler(Name, rpcServer)
	require.NoError(err)
	mux := http.NewServeMux()
	mux.Handle(Endpoint, handler)
	testServer := httptest.NewServer(mux)
	client := NewClient(testServer.URL)
	return testServer, client
}

type mockIndexer struct {
	execuredBlock *chain.ExecutedBlock
}

func (m *mockIndexer) GetBlock(blockID ids.ID) (*chain.ExecutedBlock, error) {
	if blockID == m.execuredBlock.BlockID {
		return m.execuredBlock, nil
	}
	return nil, errors.New("not found")
}

func (m *mockIndexer) GetBlockByHeight(height uint64) (*chain.ExecutedBlock, error) {
	if height == m.execuredBlock.Block.Hght {
		return m.execuredBlock, nil
	}
	return nil, errors.New("not found")
}

func (m *mockIndexer) GetLatestBlock() (*chain.ExecutedBlock, error) {
	return m.execuredBlock, nil
}

func (*mockIndexer) GetTransaction(_ ids.ID) (bool, int64, bool, fees.Dimensions, uint64, [][]byte, string, error) {
	panic("unimplemented")
}
