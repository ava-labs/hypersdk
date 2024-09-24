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
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/vm"

	staterpc "github.com/ava-labs/hypersdk/api/state"
)

// DefaultOptions provides the default set of options to include
// when constructing a new VM including the indexer, websocket,
// JSONRPC, and external subscriber options.
func NewDefaultOptions[T chain.PendingView]() []vm.Option[T] {
	return []vm.Option[T]{
		indexer.With[T](),
		ws.With[T](),
		jsonrpc.With[T](),
		externalsubscriber.With[T](),
		staterpc.With[T](),
	}
}

// New returns a VM with DefaultOptions pre-supplied
func New(
	v *version.Semantic,
	genesisFactory genesis.GenesisAndRuleFactory,
	stateManager chain.StateManager,
	actionRegistry chain.ActionRegistry[*tstate.TStateView],
	authRegistry chain.AuthRegistry,
	outputRegistry chain.OutputRegistry,
	authEngine map[uint8]vm.AuthEngine,
	options ...vm.Option[*tstate.TStateView],
) (*vm.VM[*tstate.TStateView], error) {
	options = append(options, NewDefaultOptions[*tstate.TStateView]()...)
	return vm.New(
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

// NewWithRuntime returns a VM with DefaultOptions pre-supplied
func NewWithRuntime[T chain.PendingView](
	runtimeFactory chain.ViewFactory[T],
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
	return vm.NewWithRuntime[T](
		runtimeFactory,
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
