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
	// Generate private key
	require := require.New(t)
	priv, err := GeneratePrivateKey()
	require.NoError(err)

	// Sign message
	msg := []byte("hello")
	sig, err := Sign(msg, priv)
	require.NoError(err)

	// Verify signature
	require.True(Verify(msg, priv.PublicKey(), sig))
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
	require.True(IsNormalized(new(big.Int).SetBytes(s)))
	require.True(Verify(msg, PublicKey(cpk), Signature(append(r, s...))))
}

func TestASN1ParsingUpperOrder(t *testing.T) {
	// PublicKey/Signature source: https://kjur.github.io/jsrsasign/sample/sample-ecdsa.html
	ssig := "3046022100c8e58be6d0f1a14543441f13e091befe44d6f50ae3674e52ea4a83878f52a0a60221009b4d1aeb55aec89ae2caa868c0326df2e35546d163c1ddf0e9afd65d8b3c8388"
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
	require.False(IsNormalized(new(big.Int).SetBytes(s)))
	require.False(Verify(msg, PublicKey(cpk), Signature(append(r, s...))))
}

func TestNormalization(t *testing.T) {
	// Parse uncompressed public key
	require := require.New(t)
	hpk := "04306b5d823340e69712cd1feff3b31ae48f60e6f8d62d9e4248d630969b1e7c85c7425fed4efd200c102ac1d93e5bbe37b8c027fa63bd58be298734d33bda53c3"
	rpk, err := hex.DecodeString(hpk)
	require.NoError(err)
	x, y := elliptic.Unmarshal(elliptic.P256(), rpk)
	cpk := elliptic.MarshalCompressed(elliptic.P256(), x, y)
	require.Len(cpk, PublicKeyLen)

	// Parse upper half signature
	upperHalfSig := "c8e58be6d0f1a14543441f13e091befe44d6f50ae3674e52ea4a83878f52a0a6eb55aec89ae2caa868c0326df2e35546d163c1ddf0e9afd65d8b3c83883c8388"
	rsig, err := hex.DecodeString(upperHalfSig)
	require.NoError(err)
	sig := Signature(rsig)
	s := new(big.Int).SetBytes(sig[rsLen:])

	// Ensure not normalized
	require.False(IsNormalized(s))

	// Normalize s and construct new signature
	ns := NormalizeSignature(s)
	require.True(IsNormalized(ns))
	require.Zero(ns.Cmp(NormalizeSignature(ns)))
	nsig := make([]byte, SignatureLen)
	copy(nsig[:rsLen], sig[:rsLen])
	copy(nsig[rsLen:], ns.Bytes())

	// Verify signature passes
	msg := []byte("aaa")
	require.True(Verify(msg, PublicKey(cpk), Signature(nsig)))
}
