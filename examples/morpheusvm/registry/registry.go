// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
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
		Action.Register((&actions.Transfer{}).GetTypeID(), actions.UnmarshalTransfer),

		// When registering new auth, ALWAYS make sure to append at the end.
		Auth.Register((&auth.ED25519{}).GetTypeID(), auth.UnmarshalED25519),
		Auth.Register((&auth.SECP256R1{}).GetTypeID(), auth.UnmarshalSECP256R1),
		Auth.Register((&auth.BLS{}).GetTypeID(), auth.UnmarshalBLS),
	)

	// Adding actions to ABI
	// Default wallet integration requires all actions to be in ABI to be able to sign them
	// FIXME: Would be better to integrate it with ActionRegistry
	var actionsForABI []codec.HavingTypeId = []codec.HavingTypeId{
		&actions.Transfer{},
		//... add all other actions here
	}

	var err error
	consts.ABIString, err = codec.GetVmABIString(actionsForABI)
	errs.Add(err)

	if errs.Errored() {
		panic(errs.Err)
	}
}
