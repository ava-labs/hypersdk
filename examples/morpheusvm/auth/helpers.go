// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

func GetSigner(auth chain.Auth) (codec.ShortBytes, bool) {
	switch a := auth.(type) {
	case *ED25519:
		return a.Signer.ShortBytes(), true
	case *SECP256R1:
		return a.Signer.ShortBytes(), true
	default:
		return nil, false
	}
}
