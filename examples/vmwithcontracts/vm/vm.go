// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/actions"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var (
	Action      chain.ActionRegistry
	Auth        chain.AuthRegistry
	ReturnType  chain.ReturnTypeRegistry
	wasmRuntime *runtime.WasmRuntime
)

// Setup types
func init() {
	Action = chain.NewActionRegistry()
	Auth = chain.NewAuthRegistry()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		Action.Register(&actions.Transfer{}, nil, actions.UnmarshalTransfer),
		Action.Register(&actions.Call{}, nil, actions.UnmarshalCallContract(wasmRuntime)),
		Action.Register(&actions.Publish{}, nil, actions.UnmarshalPublishContract),
		Action.Register(&actions.Deploy{}, nil, actions.UnmarshalDeployContract),

		// When registering new auth, ALWAYS make sure to append at the end.
		Auth.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		Auth.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		Auth.Register(&auth.BLS{}, auth.UnmarshalBLS),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}

// New returns a VM with the indexer, websocket, rpc, and external subscriber apis enabled.
func New(options ...vm.Option) (*vm.VM, error) {
	opts := append([]vm.Option{
		indexer.With(),
		ws.With(),
		jsonrpc.With(),
		With(), // Add Controller API
		externalsubscriber.With(),
	}, options...)

	return NewWithOptions(opts...)
}

// NewWithOptions returns a VM with the specified options
func NewWithOptions(options ...vm.Option) (*vm.VM, error) {
	opts := append([]vm.Option{
		WithRuntime(),
	}, options...)
	return vm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.StateManager{},
		Action,
		Auth,
		auth.Engines(),
		opts...,
	)
}
