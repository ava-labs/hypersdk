// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256k1

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	PublicKeyLen  = 33
	PrivateKeyLen = 32
	SignatureLen  = 65
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
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(privKey.D.Bytes()) != PrivateKeyLen {
		return GeneratePrivateKey()
	}
	return PrivateKey(privKey.D.Bytes()), nil
}

// PublicKey returns the secp256k1 public key associated with the private key.
func (p PrivateKey) PublicKey() PublicKey {
	privKey, err := crypto.ToECDSA(p[:])
	if err != nil {
		return EmptyPublicKey
	}
	return PublicKey(crypto.CompressPubkey(&privKey.PublicKey))
}

func (p PrivateKey) Scalar() [32]byte {
	var scalar [32]byte
	copy(scalar[:], p[:])
	return scalar
}

// Sign returns a valid signature for msg using pk.
func (p PrivateKey) Sign(msg []byte) Signature {
	privKey, err := crypto.ToECDSA(p[:])
	if err != nil {
		return EmptySignature
	}
	digest := crypto.Keccak256Hash(msg)
	sig, err := crypto.Sign(digest.Bytes(), privKey)
	if err != nil {
		return EmptySignature
	}
	return Signature(sig)
}

// Verify returns whether s is a valid signature of msg by p.
func Verify(msg []byte, p PublicKey, s Signature) bool {
	pk, err := crypto.DecompressPubkey(p[:])
	if err != nil {
		return false
	}
	publicKeyBytes := crypto.FromECDSAPub(pk)
	// pk, err := secp256k1.RecoverPubkey(msg, s[:])
	// if err != nil {
	// 	fmt.Println("err", err)
	// 	return false
	// }
	digest := crypto.Keccak256Hash(msg)
	signatureNoRecoverID := s[:len(s)-1]
	res := crypto.VerifySignature(publicKeyBytes[:], digest.Bytes(), signatureNoRecoverID[:])
	return res
}
func PublicKeyBytesToAddress(publicKey []byte) common.Address {
	hash := crypto.Keccak256Hash(publicKey[1:])
	address := hash.Bytes()[12:]
	return common.BytesToAddress(address)
}

func PublicKeyToAddress(p PublicKey) (common.Address, error) {
	pk, err := crypto.DecompressPubkey(p[:])
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pk), nil
}
