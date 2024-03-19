// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

const (
	HRP = "matrix"
)

func Address(pk ed25519.PublicKey) string {
	addrString, _ := address.FormatBech32(HRP, pk[:])
	return addrString
}
