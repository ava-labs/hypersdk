// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/auth/common"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"

	authbls "github.com/ava-labs/hypersdk/auth/bls"
	authed25519 "github.com/ava-labs/hypersdk/auth/ed25519"
	authsecp256r1 "github.com/ava-labs/hypersdk/auth/secp256r1"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]
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

		// When registering new auth, ALWAYS make sure to append at the end.
		AuthParser.Register(&authed25519.ED25519{}, authed25519.UnmarshalED25519),
		AuthParser.Register(&authsecp256r1.SECP256R1{}, authsecp256r1.UnmarshalSECP256R1),
		AuthParser.Register(&authbls.BLS{}, authbls.UnmarshalBLS),

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
		common.Engines(),
		options...,
	)
}
