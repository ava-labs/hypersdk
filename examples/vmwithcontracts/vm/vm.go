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
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]
	wasmRuntime  *runtime.WasmRuntime
)

// Setup types
func init() {
	ActionParser = codec.NewTypeParser[chain.Action]()
	AuthParser = codec.NewTypeParser[chain.Auth]()
	OutputParser = codec.NewTypeParser[codec.Typed]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		ActionParser.Register(&actions.Transfer{}, actions.UnmarshalTransfer),
		ActionParser.Register(&actions.Call{}, actions.UnmarshalCallContract(wasmRuntime)),
		ActionParser.Register(&actions.Publish{}, actions.UnmarshalPublishContract),
		ActionParser.Register(&actions.Deploy{}, actions.UnmarshalDeployContract),

		// When registering new auth, ALWAYS make sure to append at the end.
		AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		OutputParser.Register(&actions.Result{}, nil),
		OutputParser.Register(&actions.AddressOutput{}, nil),
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
		&storage.BalanceHandler{},
		metadata.NewDefaultManager(),
		ActionParser,
		AuthParser,
		OutputParser,
		auth.Engines(),
		opts...,
	)
}
