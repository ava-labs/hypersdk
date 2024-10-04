// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
)

type Parser struct {
	rules          genesis.RuleFactory
	actionRegistry chain.ActionRegistry
	authRegistry   chain.AuthRegistry
	outputRegistry chain.OutputRegistry
}

func NewParser(
	ruleFactory genesis.RuleFactory,
	actionRegistry chain.ActionRegistry,
	authRegistry chain.AuthRegistry,
	outputRegistry chain.OutputRegistry,
) *Parser {
	return &Parser{
		rules:          ruleFactory,
		actionRegistry: actionRegistry,
		authRegistry:   authRegistry,
		outputRegistry: outputRegistry,
	}
}

func NewEmptyParser() *Parser {
	return &Parser{
		rules:          &genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
		actionRegistry: codec.NewTypeParser[chain.Action](),
		authRegistry:   codec.NewTypeParser[chain.Auth](),
		outputRegistry: codec.NewTypeParser[codec.Typed](),
	}
}

func (p *Parser) Rules(t int64) chain.Rules {
	return p.rules.GetRules(t)
}

func (p *Parser) ActionRegistry() chain.ActionRegistry {
	return p.actionRegistry
}

func (p *Parser) AuthRegistry() chain.AuthRegistry {
	return p.authRegistry
}

func (p *Parser) OutputRegistry() chain.OutputRegistry {
	return p.outputRegistry
}
