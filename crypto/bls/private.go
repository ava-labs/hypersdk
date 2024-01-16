// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"errors"

	"github.com/ava-labs/avalanchego/utils/crypto/bls"
)

const PrivateKeyLen = bls.SecretKeyLen

var errFailedPrivateKeyDeserialize = errors.New("couldn't deserialize secret key")

type PrivateKey = bls.SecretKey

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
