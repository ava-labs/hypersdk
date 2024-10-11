// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/actions"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/consts"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]
	// global runtime which doesn't work in parallel. 
	// Better to spawn a pool of runtimes and sequence them accordingly
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
		ActionParser.Register(&actions.Transfer{}, nil),
		ActionParser.Register(&actions.Deploy{}, nil),
		ActionParser.Register(&actions.Call{}, nil),

		// When registering new auth, ALWAYS make sure to append at the end.
		AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		// OutputParser.Register(&actions.TransferResult{}, nil),
		OutputParser.Register(&actions.DeployOutput{}, nil),
		OutputParser.Register(&actions.CallOutput{}, nil),
		OutputParser.Register(&actions.TransferResult{}, nil),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}

// NewWithOptions returns a VM with the specified options
func New(options ...vm.Option) (*vm.VM, error) {
	options = append(options, With()) // Add MorpheusVM API
	return defaultvm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.StateManager{},
		ActionParser,
		AuthParser,
		OutputParser,
		auth.Engines(),
		options...,
	)
}
