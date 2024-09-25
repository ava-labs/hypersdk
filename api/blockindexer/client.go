// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txindexer

import (
	"context"
	"strings"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
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

func (c *Client) GetBlock(ctx context.Context, blkID ids.ID) (*chain.StatefulBlock, error) {
	resp := GetBlockResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getBlock",
		&GetBlockRequest{BlockID: blkID},
		&resp,
	)
	return resp.Block, err
}

func (c *Client) GetBlockByHeight(ctx context.Context, height uint64) (*chain.StatefulBlock, error) {
	resp := GetBlockResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getBlockByHeight",
		&GetBlockByHeightRequest{Height: height},
		&resp,
	)
	return resp.Block, err
}

func (c *Client) GetLatestBlock(ctx context.Context) (*chain.StatefulBlock, error) {
	resp := GetBlockResponse{}
	err := c.requester.SendRequest(
		ctx,
		"getLatestBlock",
		nil,
		&resp,
	)
	return resp.Block, err
}
