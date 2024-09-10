// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
	"errors"
	"strings"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/requester"
)

func NewJSONRPCStateClient(uri string) *JSONRPCStateClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += Endpoint
	req := requester.New(uri, api.Name)
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

	errs := make([]error, 0, len(res.Errors))
	for _, err := range res.Errors {
		var newerr error
		if err != "" {
			newerr = errors.New(err)
		}
		errs = append(errs, newerr)
	}
	return res.Values, errs, nil
}
