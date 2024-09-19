// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package defaultvm

import (
	"github.com/ava-labs/avalanchego/version"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"

	staterpc "github.com/ava-labs/hypersdk/api/state"
)

// DefaultOptions provides the default set of options to include
// when constructing a new VM including the indexer, websocket,
// JSONRPC, and external subscriber options.
func NewDefaultOptions[T chain.RuntimeInterface]() []vm.Option[T] {
	return []vm.Option[T]{
		indexer.With[T](),
		ws.With[T](),
		jsonrpc.With[T](),
		externalsubscriber.With[T](),
		staterpc.With[T](),
	}
}

// New returns a VM with DefaultOptions pre-supplied
func New[T chain.RuntimeInterface](
	runtime T,
	v *version.Semantic,
	genesisFactory genesis.GenesisAndRuleFactory,
	stateManager chain.StateManager,
	actionRegistry chain.ActionRegistry[T],
	authRegistry chain.AuthRegistry,
	outputRegistry chain.OutputRegistry,
	authEngine map[uint8]vm.AuthEngine,
	options ...vm.Option[T],
) (*vm.VM[T], error) {
	options = append(options, NewDefaultOptions[T]()...)
	return vm.New(
		runtime,
		v,
		genesisFactory,
		stateManager,
		actionRegistry,
		authRegistry,
		outputRegistry,
		authEngine,
		options...,
	)
}
