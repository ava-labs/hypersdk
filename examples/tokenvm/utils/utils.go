// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"

	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
)

func Address(pk crypto.PublicKey) string {
	return crypto.Address(consts.HRP, pk)
}

func ParseAddress(s string) (crypto.PublicKey, error) {
	return crypto.ParseAddress(consts.HRP, s)
}

func CheckBit(b uint8, v uint8) bool {
	if v > hconsts.MaxUint8Offset {
		return false
	}
	marker := uint8(1 << v)
	return b&marker != 0
}

func SetBit(b uint8, v uint8) uint8 {
	if v > hconsts.MaxUint8Offset {
		return b
	}
	marker := uint8(1 << v)
	return b | marker
}
