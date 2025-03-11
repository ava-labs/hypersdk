// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/gorilla/rpc/v2/json2"

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
		"getBlock",
		&GetBlockRequest{BlockNumber: height},
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
		"getBlock",
		&GetBlockRequest{BlockNumber: math.MaxUint64},
		&resp,
	)
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalExecutedBlock(resp.BlockBytes, parser)
}

func (c *Client) GetTxResults(ctx context.Context, txID ids.ID) (GetTxResponse, bool, error) {
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

	resp.Result.CalculateCanotoCache()
	return resp, true, nil
}

func (c *Client) GetTx(ctx context.Context, txID ids.ID, parser chain.Parser) (GetTxResponse, *chain.Transaction, bool, error) {
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
		return GetTxResponse{}, nil, false, nil
	case err != nil:
		return GetTxResponse{}, nil, false, err
	}

	var tx *chain.Transaction
	tx, err = chain.UnmarshalTx(resp.TxBytes, parser)
	if err != nil {
		return GetTxResponse{}, nil, false, fmt.Errorf("failed to unmarshal tx %s: %w", txID, err)
	}

	resp.Result.CalculateCanotoCache()
	return resp, tx, true, nil
}

func (c *Client) WaitForTransaction(ctx context.Context, txCheckInterval time.Duration, txID ids.ID) (bool, uint64, error) {
	var success bool
	var fee uint64
	if err := jsonrpc.Wait(ctx, txCheckInterval, func(ctx context.Context) (bool, error) {
		resp := GetTxResponse{}
		err := c.requester.SendRequest(
			ctx,
			"getTx",
			&GetTxRequest{TxID: txID},
			&resp,
		)
		var jsonErr *json2.Error
		if errors.As(err, &jsonErr) && jsonErr.Message == ErrTxNotFound.Error() {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		success = resp.Result.Success
		fee = resp.Result.Fee
		return true, nil
	}); err != nil {
		return false, 0, err
	}
	return success, fee, nil
}
