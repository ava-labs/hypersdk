// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"

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

func (cli *Client) GetTx(ctx context.Context, id ids.ID) (int64, bool, error) {
	resp := new(controller.GetTxReply)
	err := cli.Requester.SendRequest(
		ctx,
		"getTx",
		&controller.GetTxArgs{TxID: id},
		resp,
	)
	return resp.Timestamp, resp.Accepted, err
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
