// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"errors"

	blst "github.com/supranational/blst/bindings/go"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

const PrivateKeyLen = blst.BLST_SCALAR_BYTES

var (
	errFailedPrivateKeyDeserialize = errors.New("couldn't deserialize secret key")

	// The ciphersuite is more commonly known as G2ProofOfPossession.
	// There are two digests to ensure that message space for normal
	// signatures and the proof of possession are distinct.
	ciphersuiteSignature         = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")
	ciphersuiteProofOfPossession = []byte("BLS_POP_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")
)

type PrivateKey = blst.SecretKey

func GeneratePrivateKey() (*PrivateKey, error) {
	return bls.NewSecretKey()
}

func PrivateKeyToBytes(pk *PrivateKey) []byte {
	return bls.SecretKeyToBytes(pk)
}

func PrivateKeyFromBytes(pkBytes []byte) (*PrivateKey, error) {
	pk, err := bls.SecretKeyFromBytes(pkBytes)
	if err != nil {
		return nil, errFailedPrivateKeyDeserialize
	}
	return pk, nil
}

func PublicFromPrivateKey(pk *PrivateKey) *PublicKey {
	return bls.PublicFromSecretKey(pk)
}

func Sign(msg []byte, pk *PrivateKey) *Signature {
	return bls.Sign(pk, msg)
}

func DeserializePrivateKey(pkBytes []byte) *PrivateKey {
	return bls.DeserializeSecretKey(pkBytes)
}

func SerializePrivateKey(key *PrivateKey) []byte {
	return bls.SerializeSecretKey(key)
}