// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
)

type Factory[T chain.Action[T], A chain.Auth[A]] struct {
	genesisFactory  genesis.GenesisAndRuleFactory
	balanceHandler  chain.BalanceHandler
	metadataManager chain.MetadataManager
	authEngine      map[uint8]AuthEngine

	options []Option
}

func NewFactory[T chain.Action[T], A chain.Auth[A]](
	genesisFactory genesis.GenesisAndRuleFactory,
	balanceHandler chain.BalanceHandler,
	metadataManager chain.MetadataManager,
	authEngine map[uint8]AuthEngine,
	options ...Option,
) *Factory[T, A] {
	return &Factory[T, A]{
		genesisFactory:  genesisFactory,
		balanceHandler:  balanceHandler,
		metadataManager: metadataManager,
		authEngine:      authEngine,
		options:         options,
	}
}

func (f *Factory[T, A]) New(options ...Option) (*VM, error) {
	return New(
		f.genesisFactory,
		f.balanceHandler,
		f.metadataManager,
		f.authEngine,
		append(f.options, options...)...,
	)
}
