// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
)

var (
	Action *codec.TypeParser[chain.Action]
	Auth   *codec.TypeParser[chain.Auth]
)

// Setup types
func init() {
	Action = codec.NewTypeParser[chain.Action]()
	Auth = codec.NewTypeParser[chain.Auth]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		// Pass nil as second argument if manual marshalling isn't needed (if in doubt, you probably don't)
		Action.Register(&actions.Transfer{}, actions.UnmarshalTransfer),

		// When registering new auth, ALWAYS make sure to append at the end.
		Auth.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		Auth.Register(&auth.SECP256R1{}, auth.UnmarshalSECP256R1),
		Auth.Register(&auth.BLS{}, auth.UnmarshalBLS),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}
