// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/AnomalyFi/hypersdk/crypto"

	"github.com/AnomalyFi/hypersdk/examples/tokenvm/consts"
)

func Address(pk crypto.PublicKey) string {
	return crypto.Address(consts.HRP, pk)
}

func ParseAddress(s string) (crypto.PublicKey, error) {
	return crypto.ParseAddress(consts.HRP, s)
}
