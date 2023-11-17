// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/ava-labs/hypersdk/codec"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/consts"
)

func Address(addr codec.Address) string {
	return codec.MustAddressBech32(consts.HRP, addr)
}

func ParseAddress(address string) (codec.Address, error) {
	return codec.ParseAddressBech32(consts.HRP, address) 
}
