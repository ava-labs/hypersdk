// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
)

type Factory struct {
	genesisFactory  genesis.GenesisAndRuleFactory
	balanceHandler  chain.BalanceHandler
	metadataManager chain.MetadataManager
	actionCodec     *codec.TypeParser[chain.Action]
	authCodec       *codec.TypeParser[chain.Auth]
	outputCodec     *codec.TypeParser[codec.Typed]
	authEngine      map[uint8]AuthEngine

	options []Option
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
		genesisFactory:  genesisFactory,
		balanceHandler:  balanceHandler,
		metadataManager: metadataManager,
		actionCodec:     actionCodec,
		authCodec:       authCodec,
		outputCodec:     outputCodec,
		authEngine:      authEngine,
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
		f.authEngine,
		append(f.options, options...)...,
	)
}
