// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	authbls "github.com/ava-labs/hypersdk/auth/bls"
	authed25519 "github.com/ava-labs/hypersdk/auth/ed25519"
	authsecp256r1 "github.com/ava-labs/hypersdk/auth/secp256r1"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

// Setup types
func init() {
	consts.ActionRegistry = codec.NewTypeParser[chain.Action]()
	consts.AuthRegistry = codec.NewTypeParser[chain.Auth]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		consts.ActionRegistry.Register((&actions.Transfer{}).GetTypeID(), actions.UnmarshalTransfer),

		// When registering new auth, ALWAYS make sure to append at the end.
		consts.AuthRegistry.Register((&authed25519.ED25519{}).GetTypeID(), authed25519.UnmarshalED25519),
		consts.AuthRegistry.Register((&authsecp256r1.SECP256R1{}).GetTypeID(), authsecp256r1.UnmarshalSECP256R1),
		consts.AuthRegistry.Register((&authbls.BLS{}).GetTypeID(), authbls.UnmarshalBLS),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}
