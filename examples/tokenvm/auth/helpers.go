// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
)

func GetActor(auth chain.Auth) crypto.PublicKey {
	switch a := auth.(type) {
	case *ED25519:
		return a.Signer
	default:
		return crypto.EmptyPublicKey
	}
}

func GetSigner(auth chain.Auth) crypto.PublicKey {
	switch a := auth.(type) {
	case *ED25519:
		return a.Signer
	default:
		return crypto.EmptyPublicKey
	}
}
