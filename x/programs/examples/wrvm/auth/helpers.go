// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

func GetActor(auth chain.Auth) ed25519.PublicKey {
	switch a := auth.(type) {
	case *ED25519:
		return a.Signer
	default:
		return ed25519.EmptyPublicKey
	}
}

func GetSigner(auth chain.Auth) ed25519.PublicKey {
	switch a := auth.(type) {
	case *ED25519:
		return a.Signer
	default:
		return ed25519.EmptyPublicKey
	}
}
