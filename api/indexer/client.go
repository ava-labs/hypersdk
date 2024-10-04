// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/requester"
)

func NewClient(uri string) *Client {
	uri = strings.TrimSuffix(uri, "/")
	uri += Endpoint

	return &Client{
		requester: requester.New(uri, Name),
	}
}

type Client struct {
	requester *requester.EndpointRequester
}

// Use a separate type that only decodes the block bytes because we cannot decode block JSON
// due to Actions/Auth interfaces included in the block's transactions.
type GetBlockClientResponse struct {
	BlockBytes codec.Bytes `json:"blockBytes"`
}

func (c *Client) GetBlock(ctx context.Context, blkID ids.ID, parser chain.Parser) (*chain.ExecutedBlock, error) {
	resp := GetBlockClientResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getBlock",
		&GetBlockRequest{BlockID: blkID},
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(resp.BlockBytes, parser)
}

func (c *Client) GetBlockByHeight(ctx context.Context, height uint64, parser chain.Parser) (*chain.ExecutedBlock, error) {
	resp := GetBlockClientResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getBlockByHeight",
		&GetBlockByHeightRequest{Height: height},
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(resp.BlockBytes, parser)
}

func (c *Client) GetLatestBlock(ctx context.Context, parser chain.Parser) (*chain.ExecutedBlock, error) {
	resp := GetBlockClientResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getLatestBlock",
		nil,
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(resp.BlockBytes, parser)
}

func (c *Client) GetTx(ctx context.Context, txID ids.ID) (GetTxResponse, bool, error) {
	resp := GetTxResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getTx",
		&GetTxRequest{TxID: txID},
		&resp,
	)
	switch {
	// We use string parsing here because the JSON-RPC library we use may not
	// allows us to perform errors.Is.
	case err != nil && strings.Contains(err.Error(), ErrTxNotFound.Error()):
		return GetTxResponse{}, false, nil
	case err != nil:
		return GetTxResponse{}, false, err
	}

	return resp, true, nil
}

func (c *Client) WaitForTransaction(ctx context.Context, txCheckInterval time.Duration, txID ids.ID) (bool, uint64, error) {
	var success bool
	var fee uint64
	if err := jsonrpc.Wait(ctx, txCheckInterval, func(ctx context.Context) (bool, error) {
		response, found, err := c.GetTx(ctx, txID)
		if err != nil {
			return false, err
		}
		success = response.Success
		fee = response.Fee
		return found, nil
	}); err != nil {
		return false, 0, err
	}
	return success, fee, nil
}
