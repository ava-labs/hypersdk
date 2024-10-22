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
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/actions"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/consts"
	"github.com/ava-labs/hypersdk/examples/vmwithcontracts/storage"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var wasmRuntime *runtime.WasmRuntime

func newRegistry() (chain.Registry, error) {
	actionParser := codec.NewTypeParser[chain.Action]()
	authParser := codec.NewTypeParser[chain.Auth]()
	outputParser := codec.NewTypeParser[codec.Typed]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		actionParser.Register(&actions.Transfer{}, actions.UnmarshalTransfer),
		actionParser.Register(&actions.Call{}, actions.UnmarshalCallContract(wasmRuntime)),
		actionParser.Register(&actions.Publish{}, actions.UnmarshalPublishContract),
		actionParser.Register(&actions.Deploy{}, actions.UnmarshalDeployContract),

		// When registering new auth, ALWAYS make sure to append at the end.
		authParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		authParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		authParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		outputParser.Register(&actions.Result{}, nil),
		outputParser.Register(&actions.AddressOutput{}, nil),
	)
	if errs.Errored() {
		return nil, errs.Err
	}
	return chain.NewRegistry(actionParser, authParser, outputParser), nil
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
	registry, err := newRegistry()
	if err != nil {
		return nil, err
	}
	return vm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.StateManager{},
		registry,
		auth.Engines(),
		opts...,
	)
}
