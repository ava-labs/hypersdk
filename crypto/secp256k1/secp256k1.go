// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256k1

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/secp256k1"
	"github.com/consensys/gnark-crypto/ecc/secp256k1/ecdsa"
)

const (
	PublicKeyLen  = 64 // fp.Size (uncompressed from)
	PrivateKeyLen = 96 // fr.Size + PublicKeyLen
	SignatureLen  = 64 // 2*fr.Size
)

type (
	PublicKey  [PublicKeyLen]byte
	PrivateKey [PrivateKeyLen]byte
	Signature  [SignatureLen]byte
)

var (
	EmptyPublicKey  = [PublicKeyLen]byte{}
	EmptyPrivateKey = [PrivateKeyLen]byte{}
	EmptySignature  = [SignatureLen]byte{}
)

// GeneratePrivateKey returns a secp256k1 private key.
func GeneratePrivateKey() (PrivateKey, error) {
	// this returns a public key + private key
	privKey, err := ecdsa.GenerateKey(rand.Reader)
	if err != nil {
		return EmptyPrivateKey, err
	}
	return PrivateKey(privKey.Bytes()[:PrivateKeyLen]), nil
}

// PublicKey returns the secp256k1 public key associated with the private key.
func (p PrivateKey) PublicKey() PublicKey {

	_, g := secp256k1.Generators()

	pubKey := ecdsa.PublicKey{}
	pubKey.A.ScalarMultiplication(&g, new(big.Int).SetBytes(p[PublicKeyLen:]))
	return PublicKey(pubKey.Bytes())
}

func (p PrivateKey) Scalar() [32]byte {
	var scalar [32]byte
	copy(scalar[:], p[PublicKeyLen:])
	return scalar
}

// Sign returns a valid signature for msg using pk.
func (p PrivateKey) Sign(msg []byte) Signature {

	privKeyL := ecdsa.PrivateKey{}
	privKeyL.SetBytes(p[:])

	sig, err := privKeyL.Sign(msg, sha256.New())
	if err != nil {
		return EmptySignature
	}
	return Signature(sig)
}

// Verify returns whether s is a valid signature of msg by p.
func Verify(msg []byte, p PublicKey, s Signature) bool {
	publicKey := ecdsa.PublicKey{}
	publicKey.SetBytes(p[:])
	res, _ := publicKey.Verify(s[:], msg, sha256.New())
	return res
}
