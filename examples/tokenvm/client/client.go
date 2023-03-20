// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/client"

	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
)

type Client struct {
	*client.Client // embed standard functionality
}

// New creates a new client object.
func New(uri string) *Client {
	return &Client{client.New(consts.Name, uri)}
}

func (cli *Client) Genesis(ctx context.Context) (*genesis.Genesis, error) {
	resp := new(controller.GenesisReply)
	err := cli.Requester.SendRequest(
		ctx,
		"genesis",
		nil,
		resp,
	)
	return resp.Genesis, err
}

func (cli *Client) Tx(ctx context.Context, id ids.ID) (bool, bool, int64, error) {
	resp := new(controller.TxReply)
	err := cli.Requester.SendRequest(
		ctx,
		"tx",
		&controller.TxArgs{TxID: id},
		resp,
	)
	switch {
	// We use string parsing here because the JSON-RPC library we use may not
	// allows us to perform errors.Is.
	case err != nil && strings.Contains(err.Error(), controller.ErrTxNotFound.Error()):
		return false, false, -1, nil
	case err != nil:
		return false, false, -1, err
	}
	return true, resp.Success, resp.Timestamp, nil
}

func (cli *Client) Asset(
	ctx context.Context,
	asset ids.ID,
) (bool, []byte, uint64, string, bool, error) {
	resp := new(controller.AssetReply)
	err := cli.Requester.SendRequest(
		ctx,
		"asset",
		&controller.AssetArgs{
			Asset: asset,
		},
		resp,
	)
	switch {
	// We use string parsing here because the JSON-RPC library we use may not
	// allows us to perform errors.Is.
	case err != nil && strings.Contains(err.Error(), controller.ErrAssetNotFound.Error()):
		return false, nil, 0, "", false, nil
	case err != nil:
		return false, nil, 0, "", false, err
	}
	return true, resp.Metadata, resp.Supply, resp.Owner, resp.Warp, nil
}

func (cli *Client) Balance(ctx context.Context, addr string, asset ids.ID) (uint64, error) {
	resp := new(controller.BalanceReply)
	err := cli.Requester.SendRequest(
		ctx,
		"balance",
		&controller.BalanceArgs{
			Address: addr,
			Asset:   asset,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *Client) Orders(ctx context.Context, pair string) ([]*controller.Order, error) {
	resp := new(controller.OrdersReply)
	err := cli.Requester.SendRequest(
		ctx,
		"orders",
		&controller.OrdersArgs{
			Pair: pair,
		},
		resp,
	)
	return resp.Orders, err
}

func (cli *Client) Loan(ctx context.Context, asset ids.ID, destination ids.ID) (uint64, error) {
	resp := new(controller.LoanReply)
	err := cli.Requester.SendRequest(
		ctx,
		"loan",
		&controller.LoanArgs{
			Asset:       asset,
			Destination: destination,
		},
		resp,
	)
	return resp.Amount, err
}
