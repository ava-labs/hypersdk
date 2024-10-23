// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package defaultvm

import (
	"github.com/ava-labs/avalanchego/version"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"

	staterpc "github.com/ava-labs/hypersdk/api/state"
)

// DefaultOptions provides the default set of options to include
// when constructing a new VM including the indexer, websocket,
// JSONRPC, and external subscriber options.
func NewDefaultOptions() []vm.Option {
	return []vm.Option{
		indexer.With(),
		ws.With(),
		jsonrpc.With(),
		externalsubscriber.With(),
		staterpc.With(),
	}
}

// New returns a VM with DefaultOptions pre-supplied
func New(
	v *version.Semantic,
	genesisFactory genesis.GenesisAndRuleFactory,
	balanceHandler chain.BalanceHandler,
	metadataManager chain.MetadataManager,
	actionCodec *codec.TypeParser[chain.Action],
	authCodec *codec.TypeParser[chain.Auth],
	outputCodec *codec.TypeParser[codec.Typed],
	authEngine map[uint8]vm.AuthEngine,
	options ...vm.Option,
) (*vm.VM, error) {
	options = append(options, NewDefaultOptions()...)
	return vm.New(
		v,
		genesisFactory,
		balanceHandler,
		metadataManager,
		actionCodec,
		authCodec,
		outputCodec,
		authEngine,
		options...,
	)
}
