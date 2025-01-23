// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
)

var _ chain.Parser = ChainDefinition{}

type ChainDefinitionFactory struct {
	genesisFactory  genesis.GenesisAndRuleFactory
	balanceHandler  chain.BalanceHandler
	metadataManager chain.MetadataManager
	actionCodec     *codec.TypeParser[chain.Action]
	authCodec       *codec.TypeParser[chain.Auth]
	outputCodec     *codec.TypeParser[codec.Typed]
}

func (c ChainDefinitionFactory) NewChainDefinition(
	genesisBytes []byte,
	upgradeBytes []byte,
	networkID uint32,
	chainID ids.ID,
) (ChainDefinition, error) {
	genesis, ruleFactory, err := c.genesisFactory.Load(genesisBytes, upgradeBytes, networkID, chainID)
	if err != nil {
		return ChainDefinition{}, err
	}
	return ChainDefinition{
		Genesis:         genesis,
		GenesisBytes:    genesisBytes,
		UpgradeBytes:    upgradeBytes,
		RuleFactory:     ruleFactory,
		BalanceHandler:  c.balanceHandler,
		MetadataManager: c.metadataManager,
		actionCodec:     c.actionCodec,
		authCodec:       c.authCodec,
		outputCodec:     c.outputCodec,
	}, nil
}

type ChainDefinition struct {
	Genesis                    genesis.Genesis
	GenesisBytes, UpgradeBytes []byte
	RuleFactory                chain.RuleFactory
	BalanceHandler             chain.BalanceHandler
	MetadataManager            chain.MetadataManager
	actionCodec                *codec.TypeParser[chain.Action]
	authCodec                  *codec.TypeParser[chain.Auth]
	outputCodec                *codec.TypeParser[codec.Typed]
}

func (c ChainDefinition) Rules(t int64) chain.Rules {
	return c.RuleFactory.GetRules(t)
}

func (c ChainDefinition) ActionCodec() *codec.TypeParser[chain.Action] {
	return c.actionCodec
}

func (c ChainDefinition) OutputCodec() *codec.TypeParser[codec.Typed] {
	return c.outputCodec
}

func (c ChainDefinition) AuthCodec() *codec.TypeParser[chain.Auth] {
	return c.authCodec
}

type Factory struct {
	chainDefinitionFactory ChainDefinitionFactory

	authEngine map[uint8]AuthEngine
	options    []Option
}

func NewFactory(
	genesisFactory genesis.GenesisAndRuleFactory,
	balanceHandler chain.BalanceHandler,
	metadataManager chain.MetadataManager,
	actionCodec *codec.TypeParser[chain.Action],
	authCodec *codec.TypeParser[chain.Auth],
	outputCodec *codec.TypeParser[codec.Typed],
	authEngine map[uint8]AuthEngine,
	options ...Option,
) *Factory {
	return &Factory{
		chainDefinitionFactory: ChainDefinitionFactory{
			genesisFactory:  genesisFactory,
			balanceHandler:  balanceHandler,
			metadataManager: metadataManager,
			actionCodec:     actionCodec,
			authCodec:       authCodec,
			outputCodec:     outputCodec,
		},
		authEngine: authEngine,
		options:    options,
	}
}

func (f *Factory) New(options ...Option) (*VM, error) {
	return New(
		f.chainDefinitionFactory,
		f.authEngine,
		append(f.options, options...)...,
	)
}
