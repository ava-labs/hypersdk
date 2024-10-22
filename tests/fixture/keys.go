// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fixture

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
)

type Ed25519TestKey struct {
	PrivateKey ed25519.PrivateKey
	Address    codec.Address
}

func newDefaultKeys() []ed25519.PrivateKey {
	testKeys := make([]ed25519.PrivateKey, len(ed25519HexKeys))
	for i, keyHex := range ed25519HexKeys {
		bytes, err := codec.LoadHex(keyHex, ed25519.PrivateKeyLen)
		if err != nil {
			panic(err)
		}
		testKeys[i] = ed25519.PrivateKey(bytes)
	}

	return testKeys
}
