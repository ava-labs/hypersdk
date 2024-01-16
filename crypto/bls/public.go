// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"errors"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

const PublicKeyLen = bls.PublicKeyLen

var (
	ErrNoPublicKeys              = errors.New("no public keys")
	ErrFailedPublicKeyDecompress = errors.New("couldn't decompress public key")
)

type (
	PublicKey          = bls.PublicKey
	AggregatePublicKey = bls.AggregatePublicKey
)

func PublicKeyToBytes(pk *PublicKey) []byte {
	return bls.PublicKeyToBytes(pk)
}

func PublicKeyFromBytes(pkBytes []byte) (*PublicKey, error) {
	return bls.PublicKeyFromBytes(pkBytes)
}

func Verify(msg []byte, pk *PublicKey, sig *Signature) bool {
	return bls.Verify(pk, sig, msg)
}
