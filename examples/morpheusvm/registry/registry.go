// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

// Setup types
func init() {
	consts.ActionRegistry = codec.NewTypeParser[chain.Action]()
	consts.AuthRegistry = codec.NewTypeParser[chain.Auth]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		consts.ActionRegistry.Register((&actions.Transfer{}).GetTypeID(), actions.UnmarshalTransfer, false),

		// When registering new auth, ALWAYS make sure to append at the end.
		consts.AuthRegistry.Register((&auth.ED25519{}).GetTypeID(), auth.UnmarshalED25519, false),
		consts.AuthRegistry.Register((&auth.SECP256R1{}).GetTypeID(), auth.UnmarshalSECP256R1, false),
		consts.AuthRegistry.Register((&auth.BLS{}).GetTypeID(), auth.UnmarshalBLS, false),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}
