// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"

	"github.com/ava-labs/avalanchego/version"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]

	AuthProvider *auth.AuthProvider
)

// Setup types
func init() {
	ActionParser = codec.NewTypeParser[chain.Action]()
	AuthParser = codec.NewTypeParser[chain.Auth]()
	OutputParser = codec.NewTypeParser[codec.Typed]()
	AuthProvider = auth.NewAuthProvider()

	if err := auth.WithDefaultPrivateKeyFactories(AuthProvider); err != nil {
		panic(err)
	}

	if err := errors.Join(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		ActionParser.Register(&actions.Transfer{}, nil),

		// When registering new auth, ALWAYS make sure to append at the end.
		AuthParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		AuthParser.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		AuthParser.Register(&auth.BLS{}, auth.UnmarshalBLS),

		OutputParser.Register(&actions.TransferResult{}, nil),
	); err != nil {
		panic(err)
	}
}

// NewWithOptions returns a VM with the specified options
func New(options ...vm.Option) (*vm.VM, error) {
	options = append(options, With()) // Add MorpheusVM API
	return defaultvm.New(
		&version.Semantic{Major: 0, Minor: 0, Patch: 1},
		genesis.DefaultGenesisFactory{},
		&storage.BalanceHandler{},
		metadata.NewDefaultManager(),
		ActionParser,
		AuthParser,
		OutputParser,
		auth.Engines(),
		options...,
	)
}
