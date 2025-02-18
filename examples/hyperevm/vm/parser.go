// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
)

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
