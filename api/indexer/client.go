// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/requester"
)

func NewClient(uri string, ops ...requester.Option) *Client {
	uri = strings.TrimSuffix(uri, "/")
	uri += Endpoint

	return &Client{
		requester: requester.New(uri, Name, ops...),
	}
}

type Client struct {
	requester *requester.EndpointRequester
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
