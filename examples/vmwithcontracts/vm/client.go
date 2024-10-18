// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/requester"
	"github.com/ava-labs/hypersdk/utils"
)

const balanceCheckInterval = 500 * time.Millisecond

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

func (cli *JSONRPCClient) Balance(ctx context.Context, addr codec.Address) (uint64, error) {
	resp := new(BalanceReply)
	err := cli.requester.SendRequest(
		ctx,
		"balance",
		&BalanceArgs{
			Address: addr,
		},
		resp,
	)
	return resp.Amount, err
}

func (cli *JSONRPCClient) WaitForBalance(
	ctx context.Context,
	addr codec.Address,
	min uint64,
) error {
	return jsonrpc.Wait(ctx, balanceCheckInterval, func(ctx context.Context) (bool, error) {
		balance, err := cli.Balance(ctx, addr)
		if err != nil {
			return false, err
		}
		shouldExit := balance >= min
		if !shouldExit {
			utils.Outf(
				"{{yellow}}waiting for %s balance: %s{{/}}\n",
				utils.FormatBalance(min),
				addr,
			)
		}
		return shouldExit, nil
	})
}

func (cli *JSONRPCClient) Parser(ctx context.Context) (chain.Parser, error) {
	g, err := cli.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	return NewParser(g), nil
}

var _ chain.Parser = (*Parser)(nil)

type Parser struct {
	genesis *genesis.DefaultGenesis
}

func (p *Parser) Rules(_ int64) chain.Rules {
	return p.genesis.Rules
}

func (*Parser) ActionCodec() *codec.TypeParser[chain.Action] {
	return ActionParser
}

func (*Parser) OutputCodec() *codec.TypeParser[codec.Typed] {
	return OutputParser
}

func (*Parser) AuthCodec() *codec.TypeParser[chain.Auth] {
	return AuthParser
}

func NewParser(genesis *genesis.DefaultGenesis) chain.Parser {
	return &Parser{genesis: genesis}
}

// Used as a lambda function for creating ExternalSubscriberServer parser
func CreateParser(genesisBytes []byte) (chain.Parser, error) {
	var genesis genesis.DefaultGenesis
	if err := json.Unmarshal(genesisBytes, &genesis); err != nil {
		return nil, err
	}
	return NewParser(&genesis), nil
}
