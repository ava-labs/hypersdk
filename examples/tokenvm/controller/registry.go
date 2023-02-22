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
		consts.ActionRegistry.Register(&actions.Authorize{}, actions.UnmarshalAuthorize),
		consts.ActionRegistry.Register(&actions.Transfer{}, actions.UnmarshalTransfer),
		consts.ActionRegistry.Register(&actions.Clear{}, actions.UnmarshalClear),
		consts.ActionRegistry.Register(&actions.Index{}, actions.UnmarshalIndex),
		consts.ActionRegistry.Register(&actions.Unindex{}, actions.UnmarshalUnindex),
		consts.ActionRegistry.Register(&actions.Modify{}, actions.UnmarshalModify),

		consts.AuthRegistry.Register(&auth.Direct{}, auth.UnmarshalDirect),
		consts.AuthRegistry.Register(&auth.Delegate{}, auth.UnmarshalDelegate),
		// TODO: multi-sig
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}
