// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

// Note: Registry will error during initialization if a duplicate ID is assigned. We explicitly assign IDs to avoid accidental remapping.
const (
	ED25519Key   = "ed25519"
	Secp256r1Key = "secp256r1"
	BLSKey       = "bls"
)
