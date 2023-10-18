// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

// GetSigner extracts the signer that authorized
// the transaction. If the auth type is not supported,
// it will return nil.
func GetSigner(auth chain.Auth) codec.ShortBytes {
	switch a := auth.(type) {
	case *ED25519:
		return a.signerAddress()
	case *SECP256R1:
		return a.signerAddress()
	default:
		// We should never reach this point during block
		// execution because types should be asserted during parse.
		return nil
	}
}
