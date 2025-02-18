// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/hyperevm/actions"
	"github.com/ava-labs/hypersdk/examples/hyperevm/auth"
	"github.com/ava-labs/hypersdk/examples/hyperevm/consts"
	"github.com/ava-labs/hypersdk/examples/hyperevm/storage"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/vm"

	hauth "github.com/ava-labs/hypersdk/auth"
)

var (
	ActionParser *codec.TypeParser[chain.Action]
	AuthParser   *codec.TypeParser[chain.Auth]
	OutputParser *codec.TypeParser[codec.Typed]

	AuthProvider *hauth.AuthProvider
)

func init() {
	ActionParser = codec.NewTypeParser[chain.Action]()
	AuthParser = codec.NewTypeParser[chain.Auth]()
	OutputParser = codec.NewTypeParser[codec.Typed]()
	AuthProvider = hauth.NewAuthProvider()

	errs := &wrappers.Errs{}

	hauth.WithDefaultPrivateKeyFactories(AuthProvider, errs)

	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		ActionParser.Register(&actions.EvmCall{}, nil),
		ActionParser.Register(&actions.EvmSignedCall{}, nil),

		// When registering new auth, ALWAYS make sure to append at the end.
		AuthParser.Register(&hauth.ED25519{}, hauth.UnmarshalED25519),
		AuthParser.Register(&hauth.SECP256R1{}, hauth.UnmarshalSECP256R1),
		AuthParser.Register(&hauth.BLS{}, hauth.UnmarshalBLS),
		AuthParser.Register(&auth.SECP256K1{}, auth.UnmarshalSECP256K1),

		OutputParser.Register(&actions.EvmCallResult{}, nil),
	)

	if errs.Errored() {
		panic(errs.Err)
	}
}

func New(options ...vm.Option) (*vm.VM, error) {
	options = append(options, jsonrpc.With(), ws.With(), With())
	return vm.New(
		consts.Version,
		genesis.DefaultGenesisFactory{},
		&storage.BalanceHandler{},
		metadata.NewDefaultManager(),
		ActionParser,
		AuthParser,
		OutputParser,
		hauth.Engines(),
		options...,
	)
}
