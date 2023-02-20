// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/vm"
)

type Client struct {
	Requester *requester.EndpointRequester
}

// New creates a new client object.
func New(name string, uri string) *Client {
	req := requester.New(
		fmt.Sprintf("%s%s", uri, vm.Endpoint),
		name,
	)
	return &Client{Requester: req}
}

func (cli *Client) Ping(ctx context.Context) (bool, error) {
	resp := new(vm.PingReply)
	err := cli.Requester.SendRequest(ctx,
		"ping",
		nil,
		resp,
	)
	return resp.Success, err
}

func (cli *Client) Network(ctx context.Context) (uint32, ids.ID, ids.ID, error) {
	resp := new(vm.NetworkReply)
	err := cli.Requester.SendRequest(
		ctx,
		"network",
		nil,
		resp,
	)
	return resp.NetworkID, resp.SubnetID, resp.ChainID, err
}

func (cli *Client) Accepted(ctx context.Context) (ids.ID, uint64, error) {
	resp := new(vm.LastAcceptedReply)
	err := cli.Requester.SendRequest(
		ctx,
		"lastAccepted",
		nil,
		resp,
	)
	return resp.BlockID, resp.Height, err
}

func (cli *Client) SuggestedRawFee(ctx context.Context) (uint64, uint64, error) {
	resp := new(vm.SuggestedRawFeeReply)
	err := cli.Requester.SendRequest(
		ctx,
		"suggestedRawFee",
		nil,
		resp,
	)
	return resp.UnitPrice, resp.BlockCost, err
}

func (cli *Client) SubmitTx(ctx context.Context, d []byte) (ids.ID, error) {
	resp := new(vm.SubmitTxReply)
	err := cli.Requester.SendRequest(
		ctx,
		"submitTx",
		&vm.SubmitTxArgs{Tx: d},
		resp,
	)
	return resp.TxID, err
}

func (cli *Client) DecisionsPort(ctx context.Context) (uint16, error) {
	resp := new(vm.PortReply)
	err := cli.Requester.SendRequest(
		ctx,
		"decisionsPort",
		nil,
		resp,
	)
	return resp.Port, err
}

func (cli *Client) BlocksPort(ctx context.Context) (uint16, error) {
	resp := new(vm.PortReply)
	err := cli.Requester.SendRequest(
		ctx,
		"blocksPort",
		nil,
		resp,
	)
	return resp.Port, err
}
