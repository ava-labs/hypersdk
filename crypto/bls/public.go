// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"errors"

	blst "github.com/supranational/blst/bindings/go"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

const PublicKeyLen = blst.BLST_P1_COMPRESS_BYTES

var (
	ErrNoPublicKeys               = errors.New("no public keys")
	ErrFailedPublicKeyDecompress  = errors.New("couldn't decompress public key")
)

type (
	PublicKey          = blst.P1Affine
	AggregatePublicKey = blst.P1Aggregate
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

func DeserializePublicKey(pkBytes []byte) *PublicKey {
	return bls.DeserializePublicKey(pkBytes)
}

func SerializePublicKey(key *PublicKey) []byte {
	return bls.SerializePublicKey(key)
}