// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codectest

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
)

// NewRandomAddress returns a random address
// for use during testing
func NewRandomAddress() codec.Address {
	typeID := byte(0)
	addr := ids.GenerateTestID()
	return codec.CreateAddress(typeID, addr)
}
