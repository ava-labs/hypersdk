// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"strings"

	"github.com/ava-labs/hypersdk/requester"
)

func NewJSONRPCStateClient(uri string) *JSONRPCStateClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCStateEndpoint
	req := requester.New(uri, Name)
	return &JSONRPCStateClient{requester: req}
}

type JSONRPCStateClient struct {
	requester *requester.EndpointRequester
}

func (c *JSONRPCStateClient) ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error, error) {
	res := new(ReadStateResponse)
	err := c.requester.SendRequest(ctx, "readState", ReadStateRequest{Keys: keys}, res)
	if err != nil {
		return nil, nil, err
	}
	return res.Values, res.Errors, nil
}
