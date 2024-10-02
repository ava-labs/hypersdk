// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"
)

func newRegistryFactory() (chain.RegistryFactory, error) {
	actionParser := codec.NewTypeParser[chain.Action]()
	authParser := codec.NewTypeParser[chain.Auth]()
	outputParser := codec.NewTypeParser[codec.Typed]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		actionParser.Register(&actions.Transfer{}, actions.UnmarshalTransfer),

		// When registering new auth, ALWAYS make sure to append at the end.
		authParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		authParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		authParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		outputParser.Register(&actions.TransferResult{}, nil),
	)
	if errs.Errored() {
		return nil, errs.Err
	}
	return func() (actionRegistry chain.ActionRegistry, authRegistry chain.AuthRegistry, outputRegistry chain.OutputRegistry) {
		return actionParser, authParser, outputParser
	}, nil
}

// NewWithOptions returns a VM with the specified options
func New(options ...vm.Option) (*vm.VM, error) {
	options = append(options, With()) // Add MorpheusVM API
	registryFactory, err := newRegistryFactory()
	if err != nil {
		return nil, err
	}
	return defaultvm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.StateManager{},
		registryFactory,
		auth.Engines(),
		options...,
	)
}
