// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

// Note: Registry will error during initialization if a duplicate ID is assigned. We explicitly assign IDs to avoid accidental remapping.
const (
	// Auth TypeIDs
	ED25519ID   uint8 = 0
	SECP256R1ID uint8 = 1
	BLSID       uint8 = 2

	ED25519Key   = "ed25519"
	Secp256r1Key = "secp256r1"
	BLSKey       = "bls"
)

func DefaultEngines() Engines {
	return Engines{
		ED25519ID: &ED25519AuthEngine{},
	}
}
