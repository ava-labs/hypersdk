// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/consts"
)

func Address(pk ed25519.PublicKey) string {
	addrString, _ := address.FormatBech32(consts.HRP, pk[:])
	return addrString
}

func ParseAddress(s string) (ed25519.PublicKey, error) {
	phrp, pk, err := address.ParseBech32(s)
	if err != nil {
		return ed25519.EmptyPublicKey, err
	}
	if phrp != consts.HRP {
		return ed25519.EmptyPublicKey, codec.ErrIncorrectHRP
	}
	// The parsed public key may be greater than [PublicKeyLen] because the
	// underlying Bech32 implementation requires bytes to each encode 5 bits
	// instead of 8 (and we must pad the input to ensure we fill all bytes):
	// https://github.com/btcsuite/btcd/blob/902f797b0c4b3af3f7196d2f5d2343931d1b2bdf/btcutil/bech32/bech32.go#L325-L331
	if len(pk) < ed25519.PublicKeyLen {
		return ed25519.EmptyPublicKey, crypto.ErrInvalidPublicKey
	}
	return ed25519.PublicKey(pk[:ed25519.PublicKeyLen]), nil
}
