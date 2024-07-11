// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"github.com/ava-labs/hypersdk/vm"
)

// Note: Registry will error during initialization if a duplicate ID is assigned. We explicitly assign IDs to avoid accidental remapping.
const (
	// Auth TypeIDs
	ED25519ID   uint8 = 0
	SECP256R1ID uint8 = 1
	BLSID       uint8 = 2
)

func Engines() map[uint8]vm.AuthEngine {
	return map[uint8]vm.AuthEngine{
		ED25519ID: &ED25519AuthEngine{},
	}
}
