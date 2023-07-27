// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
)

func GetActor(auth chain.Auth) crypto.PublicKey {
	switch auth.GetTypeID() {
	case (&ED25519{}).GetTypeID():
		return auth.(*ED25519).Signer
	default:
		return crypto.EmptyPublicKey
	}
}

func GetSigner(auth chain.Auth) crypto.PublicKey {
	switch auth.GetTypeID() {
	case (&ED25519{}).GetTypeID():
		return auth.(*ED25519).Signer
	default:
		return crypto.EmptyPublicKey
	}
}
