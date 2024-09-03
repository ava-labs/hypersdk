// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package registry

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

var (
	Action *codec.TypeParser[chain.Action]
	Auth   *codec.TypeParser[chain.Auth]
)

// Setup types
func init() {
}
