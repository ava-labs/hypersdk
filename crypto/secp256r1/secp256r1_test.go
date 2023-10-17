// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256r1

import (
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratePrivateKey(t *testing.T) {
	require := require.New(t)
	priv, err := GeneratePrivateKey()
	require.NoError(err)
	require.Len(priv, PrivateKeyLen)
}

func TestSignVerify(t *testing.T) {
	require := require.New(t)
	for i := 0; i < 1000; i++ {
		// Generate private key
		priv, err := GeneratePrivateKey()
		require.NoError(err)

		// Sign message
		msg := []byte("hello")
		sig, err := Sign(msg, priv)
		require.NoError(err)

		// Verify signature
		require.True(Verify(msg, priv.PublicKey(), sig))
	}
}

func TestNormalization(t *testing.T) {
	require := require.New(t)
	for i := 0; i < 1000; i++ {
		// Generate private key
		priv, err := GeneratePrivateKey()
		require.NoError(err)

		// Sign message
		msg := []byte("hello")
		sig, err := denormalizedSign(msg, priv)
		require.NoError(err)

		// Verify signature
		r := new(big.Int).SetBytes(sig[:rsLen])
		s := new(big.Int).SetBytes(sig[rsLen:])
		require.False(normalizedS(s))
		require.False(Verify(msg, priv.PublicKey(), sig))

		// Normalize signature
		ns := normalizeS(s)

		// Verify fixed signature
		require.True(normalizedS(ns))
		require.True(Verify(msg, priv.PublicKey(), generateSignature(r, ns)))
	}
}

func TestInvalidSignature(t *testing.T) {
	msg := []byte("hello")
	publicKey := "03831f7d72b912e589ec5dec0321342f21d036a1983601926654e4d1c2278ab278"
	badSignature := "96f96b7eb69615acd55c4b9fede157ce054c1520789cbc519c67c8b43893337c37202734c76c8beb8b200103860dd5edede4cc98f4b47ba81e92e151e516572f"

	require := require.New(t)
	pk, err := hex.DecodeString(publicKey)
	require.NoError(err)
	sig, err := hex.DecodeString(badSignature)
	require.NoError(err)
	require.False(Verify(msg, PublicKey(pk), Signature(sig)))
}

func TestEmptySignature(t *testing.T) {
	msg := []byte("hello")
	publicKey := "03831f7d72b912e589ec5dec0321342f21d036a1983601926654e4d1c2278ab278"

	require := require.New(t)
	pk, err := hex.DecodeString(publicKey)
	require.NoError(err)
	require.False(Verify(msg, PublicKey(pk), EmptySignature))
}

func TestBadPublicKey(t *testing.T) {
	msg := []byte("hello")
	publicKey := "03831f7d72b912e589ec5dec0321342f21d036a1983601926654e4d1c2278ab288"
	badSignature := "96f96b7eb69615acd55c4b9fede157ce054c1520789cbc519c67c8b43893337c37202734c76c8beb8b200103860dd5edede4cc98f4b47ba81e92e151e516573f"

	require := require.New(t)
	pk, err := hex.DecodeString(publicKey)
	require.NoError(err)
	sig, err := hex.DecodeString(badSignature)
	require.NoError(err)
	require.False(Verify(msg, PublicKey(pk), Signature(sig)))
}

func TestEmptyPublicKey(t *testing.T) {
	msg := []byte("hello")
	badSignature := "96f96b7eb69615acd55c4b9fede157ce054c1520789cbc519c67c8b43893337c37202734c76c8beb8b200103860dd5edede4cc98f4b47ba81e92e151e516573f"

	require := require.New(t)
	sig, err := hex.DecodeString(badSignature)
	require.NoError(err)
	require.False(Verify(msg, EmptyPublicKey, Signature(sig)))
}

func TestSignatureLargeS(t *testing.T) {
	msg := []byte("hello")
	publicKey := "03ca7eedb087dd30ba97c468f1c3ac06e0571dd055d0cdaf4147d11df0337c11f9"
	badSignature := "cd0fb02caaac3896f9a1257f077344b3710626c59913ae0100fcc2f8005b10c19d0c237bc8929f3a07993d2c42037bcee6adf33feb3523c17c9ee29b651c7ef7"

	require := require.New(t)
	pk, err := hex.DecodeString(publicKey)
	require.NoError(err)
	sig, err := hex.DecodeString(badSignature)
	require.NoError(err)
	require.False(Verify(msg, PublicKey(pk), Signature(sig)))
}

func TestASN1Parsing(t *testing.T) {
	// PublicKey/Signature source: https://kjur.github.io/jsrsasign/sample/sample-ecdsa.html
	ssig := "304502210099086f10c8fc0b32e83f6f0280997950b4f6fe376479334fab2ab12f652b767a02203c53f459a2c35ee274cff20e27461f919dd891475dcd59bab7e2e5db3bad05be"
	hpk := "04306b5d823340e69712cd1feff3b31ae48f60e6f8d62d9e4248d630969b1e7c85c7425fed4efd200c102ac1d93e5bbe37b8c027fa63bd58be298734d33bda53c3"

	// Parse uncompressed public key
	require := require.New(t)
	rpk, err := hex.DecodeString(hpk)
	require.NoError(err)
	x, y := elliptic.Unmarshal(elliptic.P256(), rpk)
	cpk := elliptic.MarshalCompressed(elliptic.P256(), x, y)
	require.Len(cpk, PublicKeyLen)

	// Parse signature
	sig, err := hex.DecodeString(ssig)
	require.NoError(err)
	r, s, err := ParseASN1Signature(sig)
	require.NoError(err)
	require.Len(r, rsLen)
	require.Len(s, rsLen)

	// Verify signature
	msg := []byte("aaa")
	require.True(Verify(msg, PublicKey(cpk), Signature(append(r, s...))))
}
