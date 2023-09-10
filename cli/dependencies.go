// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cli

import (
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

type Controller interface {
	DatabasePath() string
	Symbol() string
	Decimals() uint8
	Address(ed25519.PublicKey) string
	ParseAddress(string) (ed25519.PublicKey, error)
}
