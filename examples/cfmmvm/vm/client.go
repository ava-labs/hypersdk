// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"strings"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/consts"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/requester"
)

type JSONRPCClient struct {
	requester *requester.EndpointRequester
	g         *genesis.DefaultGenesis
}

// NewJSONRPCClient creates a new client object.
func NewJSONRPCClient(uri string) *JSONRPCClient {
	uri = strings.TrimSuffix(uri, "/")
	uri += JSONRPCEndpoint
	req := requester.New(uri, consts.Name)
	return &JSONRPCClient{req, nil}
}

func (cli *JSONRPCClient) Genesis(ctx context.Context) (*genesis.DefaultGenesis, error) {
	if cli.g != nil {
		return cli.g, nil
	}

	resp := new(GenesisReply)
	err := cli.requester.SendRequest(
		ctx,
		"genesis",
		nil,
		resp,
	)
	if err != nil {
		return nil, err
	}
	cli.g = resp.Genesis
	return resp.Genesis, nil
}

func (cli *JSONRPCClient) GetTokenInfo(ctx context.Context, tokenAddress codec.Address) (*GetTokenInfoReply, error) {
	resp := new(GetTokenInfoReply)
	err := cli.requester.SendRequest(
		ctx,
		"getTokenInfo",
		&GetTokenInfoArgs{
			TokenAddress: tokenAddress,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) GetBalance(ctx context.Context, tokenAddress codec.Address, account codec.Address) (*GetBalanceReply, error) {
	resp := new(GetBalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"getBalance",
		&GetBalanceArgs{
			TokenAddress: tokenAddress,
			Account:      account,
		},
		resp,
	)
	return resp, err
}

func (cli *JSONRPCClient) GetLiquidityPool(ctx context.Context, liquidityPoolAddress codec.Address) (*GetLiquidityPoolReply, error) {
	resp := new(GetLiquidityPoolReply)
	err := cli.requester.SendRequest(
		ctx,
		"getLiquidityPool",
		&GetLiquidityPoolArgs{
			LiquidityPoolAddress: liquidityPoolAddress,
		},
		resp,
	)
	return resp, err
}
