// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/hypersdk/crypto"
)

type Controller interface {
	DatabasePath() string
	Symbol() string
	Address(crypto.PublicKey) string
	ParseAddress(string) (crypto.PublicKey, error)
}
