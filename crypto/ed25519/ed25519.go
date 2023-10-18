// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"crypto/rand"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519/extra/cache"

	"github.com/ava-labs/hypersdk/crypto"
)

type (
	PublicKey  [ed25519.PublicKeySize]byte
	PrivateKey [ed25519.PrivateKeySize]byte
	Signature  [ed25519.SignatureSize]byte
)

const (
	PublicKeyLen  = ed25519.PublicKeySize
	PrivateKeyLen = ed25519.PrivateKeySize
	// PrivateKeySeedLen is defined because ed25519.PrivateKey
	// is formatted as privateKey = seed|publicKey. We use this const
	// to extract the publicKey below.
	PrivateKeySeedLen = ed25519.SeedSize
	SignatureLen      = ed25519.SignatureSize

	// TODO: make this tunable
	MinBatchSize = 16

	// TODO: make this tunable
	cacheSize = 128_000 // ~179MB (keys are ~1.4KB each)
)

var (
	EmptyPublicKey  = [ed25519.PublicKeySize]byte{}
	EmptyPrivateKey = [ed25519.PrivateKeySize]byte{}
	EmptySignature  = [ed25519.SignatureSize]byte{}

	verifyOptions ed25519.Options
	cacheVerifier *Verifier
)

func init() {
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
	verifyOptions.Verify = ed25519.VerifyOptionsZIP_215

	// cacheVerifier stores expanded ed25519 Public Keys (each is ~1.4KB). Using
	// a cached expanded key reduces verification latency by ~25%.
	cacheVerifier = NewVerifier(cache.NewLRUCache(cacheSize))
}

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
	return cacheVerifier.VerifyWithOptions(p[:], msg, s[:], &verifyOptions)
}

func CachePublicKey(p PublicKey) {
	cacheVerifier.AddPublicKey(p[:])
}

type Batch struct {
	bv *ed25519.BatchVerifier
}

func NewBatch(numItems int) *Batch {
	if numItems <= 0 {
		return &Batch{ed25519.NewBatchVerifier()}
	}
	return &Batch{ed25519.NewBatchVerifierWithCapacity(numItems)}
}

func (b *Batch) Add(msg []byte, p PublicKey, s Signature) {
	cacheVerifier.AddWithOptions(b.bv, p[:], msg, s[:], &verifyOptions)
}

func (b *Batch) Verify() bool {
	return b.bv.VerifyBatchOnly(rand.Reader)
}

func (b *Batch) VerifyAsync() func() error {
	return func() error {
		if !b.Verify() {
			return crypto.ErrInvalidSignature
		}
		return nil
	}
}
