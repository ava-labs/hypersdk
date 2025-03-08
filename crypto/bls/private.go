// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"errors"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/crypto/bls/signer/localsigner"

	blst "github.com/supranational/blst/bindings/go"
)

const PrivateKeyLen = blst.BLST_SCALAR_BYTES

var errFailedPrivateKeyDeserialize = errors.New("couldn't deserialize secret key")

type PrivateKey = localsigner.LocalSigner

func GeneratePrivateKey() (*PrivateKey, error) {
	return localsigner.New()
}

func PrivateKeyToBytes(pk *PrivateKey) []byte {
	return pk.ToBytes()
}

func PrivateKeyFromBytes(pkBytes []byte) (*PrivateKey, error) {
	pk, err := localsigner.FromBytes(pkBytes)
	if err != nil {
		return nil, errFailedPrivateKeyDeserialize
	}
	return pk, nil
}

func PublicFromPrivateKey(pk *PrivateKey) *PublicKey {
	return pk.PublicKey()
}

func Sign(msg []byte, pk *PrivateKey) (*bls.Signature, error) {
	return pk.Sign(msg)
}
