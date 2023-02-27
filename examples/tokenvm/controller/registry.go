// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package controller

import (
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
)

// Setup types
func init() {
	consts.ActionRegistry = codec.NewTypeParser[chain.Action]()
	consts.AuthRegistry = codec.NewTypeParser[chain.Auth]()

	errs := &wrappers.Errs{}
	errs.Add(
		// When registering new actions, ALWAYS make sure to append at the end.
		consts.ActionRegistry.Register(&actions.Transfer{}, actions.UnmarshalTransfer),

		consts.ActionRegistry.Register(&actions.CreateAsset{}, actions.UnmarshalCreateAsset),
		consts.ActionRegistry.Register(&actions.MintAsset{}, actions.UnmarshalMintAsset),
		consts.ActionRegistry.Register(&actions.BurnAsset{}, actions.UnmarshalBurnAsset),
		consts.ActionRegistry.Register(&actions.ModifyAsset{}, actions.UnmarshalModifyAsset),

		consts.ActionRegistry.Register(&actions.CreateOrder{}, actions.UnmarshalCreateOrder),
		consts.ActionRegistry.Register(&actions.FillOrder{}, actions.UnmarshalFillOrder),
		consts.ActionRegistry.Register(&actions.CloseOrder{}, actions.UnmarshalCloseOrder),

		// When registering new auth, ALWAYS make sure to append at the end.
		consts.AuthRegistry.Register(&auth.ED25519{}, auth.UnmarshalED25519),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}
