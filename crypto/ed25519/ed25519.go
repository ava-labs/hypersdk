// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519/extra/cache"

	"github.com/ava-labs/avalanchego/utils/formatting/address"
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

// Address returns a Bech32 address from hrp and p.
// This function uses avalanchego's FormatBech32 function.
func Address(hrp string, p PublicKey) string {
	// TODO: handle error
	addrString, _ := address.FormatBech32(hrp, p[:])
	return addrString
}

// ParseAddress parses a Bech32 encoded address string and extracts
// its public key. If there is an error reading the address or the hrp
// value is not valid, ParseAddress returns an EmptyPublicKey and error.
func ParseAddress(hrp, saddr string) (PublicKey, error) {
	phrp, pk, err := address.ParseBech32(saddr)
	if err != nil {
		return EmptyPublicKey, err
	}
	if phrp != hrp {
		return EmptyPublicKey, crypto.ErrIncorrectHrp
	}
	// The parsed public key may be greater than [PublicKeyLen] because the
	// underlying Bech32 implementation requires bytes to each encode 5 bits
	// instead of 8 (and we must pad the input to ensure we fill all bytes):
	// https://github.com/btcsuite/btcd/blob/902f797b0c4b3af3f7196d2f5d2343931d1b2bdf/btcutil/bech32/bech32.go#L325-L331
	if len(pk) < PublicKeyLen {
		return EmptyPublicKey, crypto.ErrInvalidPublicKey
	}
	return PublicKey(pk[:PublicKeyLen]), nil
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

// ToHex converts a PrivateKey to a hex string.
func (p PrivateKey) ToHex() string {
	return hex.EncodeToString(p[:])
}

// Save writes [PrivateKey] to a file [filename]. If filename does
// not exist, it creates a new file with read/write permissions (0o600).
func (p PrivateKey) Save(filename string) error {
	return os.WriteFile(filename, p[:], 0o600)
}

// LoadKey returns a PrivateKey from a file filename.
// If there is an error reading the file, or the file contains an
// invalid PrivateKey, LoadKey returns an EmptyPrivateKey and an error.
func LoadKey(filename string) (PrivateKey, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(bytes) != ed25519.PrivateKeySize {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
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

// HexToKey Converts a hexadecimal encoded key into a PrivateKey. Returns
// an EmptyPrivateKey and error if key is invalid.
func HexToKey(key string) (PrivateKey, error) {
	bytes, err := hex.DecodeString(key)
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(bytes) != ed25519.PrivateKeySize {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
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
