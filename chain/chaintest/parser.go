// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"errors"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
)

type Parser struct {
	rules       chain.RuleFactory
	actionCodec *codec.TypeParser[chain.Action]
	authCodec   *codec.TypeParser[chain.Auth]
	outputCodec *codec.TypeParser[codec.Typed]
}

func NewParser(
	rules chain.RuleFactory,
	actionCodec *codec.TypeParser[chain.Action],
	authCodec *codec.TypeParser[chain.Auth],
	outputCodec *codec.TypeParser[codec.Typed],
) *Parser {
	return &Parser{
		rules:       rules,
		actionCodec: actionCodec,
		authCodec:   authCodec,
		outputCodec: outputCodec,
	}
}

func NewTestParser() *Parser {
	actionCodec := codec.NewTypeParser[chain.Action]()
	authCodec := codec.NewTypeParser[chain.Auth]()
	outputCodec := codec.NewTypeParser[codec.Typed]()

	err := errors.Join(
		actionCodec.Register(&TestAction{}, nil),
		authCodec.Register(&TestAuth{}, UnmarshalAuth),
		outputCodec.Register(&TestOutput{}, nil),
	)
	if err != nil {
		panic(err)
	}

	return NewParser(
		&genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
		actionCodec,
		authCodec,
		outputCodec,
	)
}

func (p *Parser) Rules(t int64) chain.Rules {
	return p.rules.GetRules(t)
}

func (p *Parser) ActionCodec() *codec.TypeParser[chain.Action] {
	return p.actionCodec
}

func (p *Parser) AuthCodec() *codec.TypeParser[chain.Auth] {
	return p.authCodec
}

func (p *Parser) OutputCodec() *codec.TypeParser[codec.Typed] {
	return p.outputCodec
}
