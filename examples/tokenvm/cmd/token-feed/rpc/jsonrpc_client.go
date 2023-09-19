// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"strings"

	"github.com/ava-labs/hypersdk/examples/tokenvm/cmd/token-feed/manager"
	"github.com/ava-labs/hypersdk/requester"
)

const (
	JSONRPCEndpoint = "/feed"
)

type JSONRPCClient struct {
	requester *requester.EndpointRequester
}

// New creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, "feed")
	return &JSONRPCClient{
		requester: req,
	}
}

func (cli *JSONRPCClient) FeedInfo(ctx context.Context) (string, uint64, error) {
	resp := new(FeedInfoReply)
	err := cli.requester.SendRequest(
		ctx,
		"feedInfo",
		nil,
		resp,
	)
	return resp.Address, resp.Fee, err
}

func (cli *JSONRPCClient) Feed(ctx context.Context) ([]*manager.FeedObject, error) {
	resp := new(FeedReply)
	err := cli.requester.SendRequest(
		ctx,
		"feed",
		nil,
		resp,
	)
	return resp.Feed, err
}
