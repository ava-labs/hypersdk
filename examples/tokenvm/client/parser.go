// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package client

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"

	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	_ "github.com/ava-labs/hypersdk/examples/tokenvm/controller" // ensure registry populated
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
)

var _ chain.Parser = (*Parser)(nil)

type Parser struct {
	chainID ids.ID
	genesis *genesis.Genesis
}

func (p *Parser) ChainID() ids.ID {
	return p.chainID
}

func (p *Parser) Rules(t int64) chain.Rules {
	return p.genesis.Rules(t)
}

func (*Parser) Registry() (chain.ActionRegistry, chain.AuthRegistry) {
	return consts.ActionRegistry, consts.AuthRegistry
}

func (cli *Client) Parser(ctx context.Context) (chain.Parser, error) {
	// Gather chain metadata
	g, err := cli.Genesis(ctx)
	if err != nil {
		return nil, err
	}
	_, _, chainID, err := cli.Network(ctx) // TODO: store in object to fetch less frequently
	if err != nil {
		return nil, err
	}
	return &Parser{chainID, g}, nil
}
