// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/hypersdk/codec"
)

type Controller interface {
	DatabasePath() string
	Symbol() string
	Decimals() uint8
	Address(codec.Address) string
	ParseAddress(string) (codec.Address, error)
}
