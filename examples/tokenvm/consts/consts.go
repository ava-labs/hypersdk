// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package consts

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

const (
	HRP      = "token"
	Name     = "tokenvm"
	Symbol   = "TKN"
	Decimals = 9
)

var ID ids.ID

func init() {
	vmID, err := ids.FromString("tHBYNu8ikqo4MWMHehC9iKB9mR5tB3DWzbkYmTfe9buWQ5GZ8")
	if err != nil {
		panic(err)
	}
	ID = vmID
}

// Instantiate registry here so it can be imported by any package. We set these
// values in [controller/registry].
var (
	ActionRegistry *codec.TypeParser[chain.Action, *warp.Message, bool]
	AuthRegistry   *codec.TypeParser[chain.Auth, *warp.Message, bool]
)
