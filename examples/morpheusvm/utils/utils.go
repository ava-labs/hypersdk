// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/ava-labs/hypersdk/crypto/ed25519"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

func Address(pk ed25519.PublicKey) string {
	return ed25519.Address(consts.HRP, pk)
}

func ParseAddress(s string) (ed25519.PublicKey, error) {
	return ed25519.ParseAddress(consts.HRP, s)
}
