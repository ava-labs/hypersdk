// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
)

func NewED25519Address(pk ed25519.PublicKey) codec.Address {
	return codec.CreateAddress(ED25519ID, utils.ToID(pk[:]))
}