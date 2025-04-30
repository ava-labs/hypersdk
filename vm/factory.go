// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
)

type Factory struct {
	genesisFactory  genesis.GenesisAndRuleFactory
	balanceHandler  chain.BalanceHandler
	metadataManager chain.MetadataManager
	actionCodec     *codec.CanotoParser[chain.Action]
	authCodec       *codec.CanotoParser[chain.Auth]
	outputCodec     *codec.CanotoParser[codec.Typed]
	authEngines     auth.Engines

	options []Option
}

func NewFactory(
	genesisFactory genesis.GenesisAndRuleFactory,
	balanceHandler chain.BalanceHandler,
	metadataManager chain.MetadataManager,
	actionCodec *codec.CanotoParser[chain.Action],
	authCodec *codec.CanotoParser[chain.Auth],
	outputCodec *codec.CanotoParser[codec.Typed],
	authEngines auth.Engines,
	options ...Option,
) *Factory {
	return &Factory{
		genesisFactory:  genesisFactory,
		balanceHandler:  balanceHandler,
		metadataManager: metadataManager,
		actionCodec:     actionCodec,
		authCodec:       authCodec,
		outputCodec:     outputCodec,
		authEngines:     authEngines,
		options:         options,
	}
}

func (f *Factory) New(options ...Option) (*VM, error) {
	return New(
		f.genesisFactory,
		f.balanceHandler,
		f.metadataManager,
		f.actionCodec,
		f.authCodec,
		f.outputCodec,
		f.authEngines,
		append(f.options, options...)...,
	)
}
