// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"crypto/ed25519"

	"github.com/hdevalence/ed25519consensus"

	"github.com/ava-labs/hypersdk/crypto"
)

type (
	PublicKey  [ed25519.PublicKeySize]byte
	PrivateKey [ed25519.PrivateKeySize]byte
	Signature  [ed25519.SignatureSize]byte
)

// We use the ZIP-215 specification for ed25519 signature
// verification (https://zips.z.cash/zip-0215) because it provides
// an explicit validity criteria for signatures, supports batch
// verification, and is broadly compatible with signatures produced
// by almost all ed25519 implementations (which don't require
// canonically-encoded points).
//
// You can read more about the rationale for ZIP-215 here:
// https://hdevalence.ca/blog/2020-10-04-its-25519am
//
// You can read more about the challenge of ed25519 verification here:
// https://eprint.iacr.org/2020/1244.pdf
const (
	PublicKeyLen  = ed25519.PublicKeySize
	PrivateKeyLen = ed25519.PrivateKeySize
	// PrivateKeySeedLen is defined because ed25519.PrivateKey
	// is formatted as privateKey = seed|publicKey. We use this const
	// to extract the publicKey below.
	PrivateKeySeedLen = ed25519.SeedSize
	SignatureLen      = ed25519.SignatureSize

	// TODO: make this tunable
	MinBatchSize = 4
)

var (
	EmptyPublicKey  = [ed25519.PublicKeySize]byte{}
	EmptyPrivateKey = [ed25519.PrivateKeySize]byte{}
	EmptySignature  = [ed25519.SignatureSize]byte{}
)

// GeneratePrivateKey returns a Ed25519 PrivateKey.
func GeneratePrivateKey() (PrivateKey, error) {
	_, k, err := ed25519.GenerateKey(nil)
	if err != nil {
		return EmptyPrivateKey, err
	}
	return PrivateKey(k), nil
}

// PublicKey returns a PublicKey associated with the Ed25519 PrivateKey p.
// The PublicKey is the last 32 bytes of p.
func (p PrivateKey) PublicKey() PublicKey {
	return PublicKey(p[PrivateKeySeedLen:])
}

// Sign returns a valid signature for msg using pk.
func Sign(msg []byte, pk PrivateKey) Signature {
	sig := ed25519.Sign(pk[:], msg)
	return Signature(sig)
}

// Verify returns whether s is a valid signature of msg by p.
func Verify(msg []byte, p PublicKey, s Signature) bool {
	return ed25519consensus.Verify(p[:], msg, s[:])
}

type Batch struct {
	bv ed25519consensus.BatchVerifier
}

func NewBatch(size int) *Batch {
	return &Batch{bv: ed25519consensus.NewPreallocatedBatchVerifier(size)}
}

func (b *Batch) Add(msg []byte, p PublicKey, s Signature) {
	b.bv.Add(p[:], msg, s[:])
}

func (b *Batch) Verify() bool {
	return b.bv.Verify()
}

func (b *Batch) VerifyAsync() func() error {
	return func() error {
		if !b.Verify() {
			return crypto.ErrInvalidSignature
		}
		return nil
	}
}
