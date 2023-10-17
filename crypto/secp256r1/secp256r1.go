// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256r1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/hypersdk/crypto"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

const (
	PublicKeyLen  = 33 // compressed form (SEC 1, Version 2.0, Section 2.3.3)
	PrivateKeyLen = 32
	SignatureLen  = 64 // R || S

	rsLen = 32
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

// secp256r1Order returns the curve order for the secp256r1 (P-256) curve.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
var secp256r1Order = elliptic.P256().Params().N

// secp256r1HalfOrder returns half the curve order of the secp256r1 (P-256) curve.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
var secp256r1HalfOrder = new(big.Int).Div(secp256r1Order, big.NewInt(2))

// normalized returns true if [s] falls in the lower half of the curve order (inclusive).
// This should be used when verifying signatures to ensure they are not malleable.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
func normalizedS(s *big.Int) bool {
	return s.Cmp(secp256r1HalfOrder) != 1
}

// normalizeS inverts [s] if it is not in the lower half of the curve order.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
func normalizeS(s *big.Int) *big.Int {
	if normalizedS(s) {
		return s
	}
	return new(big.Int).Sub(secp256r1Order, s)
}

// GeneratePrivateKey returns a secp256r1 PrivateKey.
func GeneratePrivateKey() (PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return EmptyPrivateKey, err
	}

	// We aren't always guaranteed that this will be 32 bytes,
	// so we use fill.
	b := make([]byte, PrivateKeyLen)
	k.D.FillBytes(b)
	return PrivateKey(b), nil
}

// generateCoordintes recovers the public key coordinates
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L120-L121
func (p PrivateKey) generateCoordinates() (*big.Int, *big.Int) {
	return elliptic.P256().ScalarBaseMult(p[:])
}

// PublicKey returns a PublicKey associated with the secp256r1 PrivateKey p.
func (p PrivateKey) PublicKey() PublicKey {
	x, y := p.generateCoordinates()

	// Output the compressed form of the PublicKey
	return PublicKey(elliptic.MarshalCompressed(elliptic.P256(), x, y))
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
	if len(bytes) != PrivateKeyLen {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
}

// generateSignature creates a valid signature, potentially padding
// r and/or s with zeros.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L39-L50
func generateSignature(r, s *big.Int) Signature {
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sigBytes := make([]byte, SignatureLen)
	// 0 pad the byte arrays from the left if they aren't big enough
	copy(sigBytes[rsLen-len(rBytes):rsLen], rBytes)
	copy(sigBytes[SignatureLen-len(sBytes):SignatureLen], sBytes)
	return Signature(sigBytes)
}

// Sign returns a valid signature for msg using pk.
//
// This function also adjusts [s] to be in the lower
// half of the curve order.
//
// This function produces signatures of equivalent
// security as RFC6979 deterministic nonce generation
// without giving up signature randomness.
//
// source: https://cs.opensource.google/go/go/+/refs/tags/go1.21.3:src/crypto/ecdsa/ecdsa.go;l=409-452
func Sign(msg []byte, pk PrivateKey) (Signature, error) {
	// Parse PrivateKey
	x, y := pk.generateCoordinates()
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		},
		D: new(big.Int).SetBytes(pk[:]),
	}

	// Sign message
	digest := sha256.Sum256(msg)
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return EmptySignature, err
	}

	// Construct signature
	ns := normalizeS(s)
	return generateSignature(r, ns), nil
}

// Verify returns whether sig is a valid signature of msg by p.
//
// The value of [s] in [sig] must be in the lower half of the curve
// order for the signature to be considered valid.
func Verify(msg []byte, p PublicKey, sig Signature) bool {
	// Perform sanity checks
	if len(p) != PublicKeyLen {
		fmt.Println("invalid pk len")
		return false
	}
	if len(sig) != SignatureLen {
		fmt.Println("invalid sig len")
		return false
	}

	// Parse PublicKey
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), p[:])
	if x == nil || y == nil {
		// This can happen if the point is not in compressed form, not
		// on the curve, or is at infinity.
		//
		// source: https://cs.opensource.google/go/go/+/refs/tags/go1.21.3:src/crypto/elliptic/elliptic.go;l=147-149
		return false
	}
	pk := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Parse Signature
	r := new(big.Int).SetBytes(sig[:rsLen])
	s := new(big.Int).SetBytes(sig[rsLen:])

	// Check if s is normalized
	if !normalizedS(s) {
		return false
	}

	// Check if signature is valid
	digest := sha256.Sum256(msg)
	return ecdsa.Verify(pk, digest[:], r, s)
}

// HexToKey Converts a hexadecimal encoded key into a PrivateKey. Returns
// an EmptyPrivateKey and error if key is invalid.
func HexToKey(key string) (PrivateKey, error) {
	bytes, err := hex.DecodeString(key)
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(bytes) != PrivateKeyLen {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
}

// ParseASN1Signature parses an ASN.1 encoded (using DER serialization) secp256r1 signature.
// This function does not normalize the extracted signature.
//
// source: https://cs.opensource.google/go/go/+/refs/tags/go1.21.3:src/crypto/ecdsa/ecdsa.go;l=549
func ParseASN1Signature(sig []byte) (r, s []byte, err error) {
	var inner cryptobyte.String
	input := cryptobyte.String(sig)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(&r) ||
		!inner.ReadASN1Integer(&s) ||
		!inner.Empty() {
		return nil, nil, errors.New("invalid ASN.1")
	}
	return r, s, nil
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

// used for testing
func denormalizedSign(msg []byte, pk PrivateKey) (Signature, error) {
	for {
		// Parse PrivateKey
		x, y := pk.generateCoordinates()
		priv := &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     x,
				Y:     y,
			},
			D: new(big.Int).SetBytes(pk[:]),
		}

		// Sign message
		digest := sha256.Sum256(msg)
		r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
		if err != nil {
			return EmptySignature, err
		}

		// Construct signature
		if !normalizedS(s) {
			return generateSignature(r, s), nil
		}
	}
}
