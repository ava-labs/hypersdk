// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"crypto/ed25519"
	"crypto/rand"
	"strconv"
	"testing"

	oed25519 "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519/extra/cache"

	"github.com/stretchr/testify/require"
)

var (
	TestPrivateKey = PrivateKey(
		[PrivateKeyLen]byte{
			32, 241, 118, 222, 210, 13, 164, 128, 3, 18,
			109, 215, 176, 215, 168, 171, 194, 181, 4, 11,
			253, 199, 173, 240, 107, 148, 127, 190, 48, 164,
			12, 48, 115, 50, 124, 153, 59, 53, 196, 150, 168,
			143, 151, 235, 222, 128, 136, 161, 9, 40, 139, 85,
			182, 153, 68, 135, 62, 166, 45, 235, 251, 246, 69, 7,
		},
	)
	TestPublicKey = []byte{
		115, 50, 124, 153, 59, 53, 196, 150, 168, 143, 151, 235,
		222, 128, 136, 161, 9, 40, 139, 85, 182, 153, 68, 135,
		62, 166, 45, 235, 251, 246, 69, 7,
	}
)

func TestGeneratePrivateKeyFormat(t *testing.T) {
	require := require.New(t)
	priv, err := GeneratePrivateKey()
	require.NoError(err, "Error Generating PrivateKey")
	require.NotEqual(priv, EmptyPrivateKey, "PrivateKey is empty")
	require.Len(priv, PrivateKeyLen, "PrivateKey has incorrect length")
}

func TestGeneratePrivateKeyDifferent(t *testing.T) {
	require := require.New(t)
	const numKeysToGenerate int = 10
	pks := [numKeysToGenerate]PrivateKey{}

	// generate keys
	for i := 0; i < numKeysToGenerate; i++ {
		priv, err := GeneratePrivateKey()
		pks[i] = priv
		require.NoError(err, "Error Generating Private Key")
	}

	// make sure keys are different
	m := make(map[PrivateKey]bool)
	for _, priv := range pks {
		require.False(m[priv], "Duplicate PrivateKey generated")
		m[priv] = true
	}
}

func TestPublicKeyValid(t *testing.T) {
	require := require.New(t)
	// Hardcoded test values
	var expectedPubKey PublicKey
	copy(expectedPubKey[:], TestPublicKey)
	pubKey := TestPrivateKey.PublicKey()
	require.Equal(expectedPubKey, pubKey, "PublicKey not equal to Expected PublicKey")
}

func TestPublicKeyFormat(t *testing.T) {
	require := require.New(t)
	priv, err := GeneratePrivateKey()
	require.NoError(err, "Error during call to GeneratePrivateKey")
	pubKey := priv.PublicKey()
	require.NotEqual(pubKey, EmptyPublicKey, "PublicKey is empty")
	require.Len(pubKey, PublicKeyLen, "PublicKey has incorrect length")
}

func TestSignSignatureValid(t *testing.T) {
	require := require.New(t)

	msg := []byte("msg")
	// Sign using ed25519
	ed25519Sign := ed25519.Sign(TestPrivateKey[:], msg)
	var expectedSig Signature
	copy(expectedSig[:], ed25519Sign)
	// Sign using crypto
	sig := Sign(msg, TestPrivateKey)
	require.Equal(sig, expectedSig, "Signature was incorrect")
}

func TestVerifyValidParams(t *testing.T) {
	require := require.New(t)
	msg := []byte("msg")
	sig := Sign(msg, TestPrivateKey)
	require.True(Verify(msg, TestPrivateKey.PublicKey(), sig),
		"Signature was invalid")
}

func TestVerifyInvalidParams(t *testing.T) {
	require := require.New(t)

	msg := []byte("msg")

	difMsg := []byte("diff msg")
	sig := Sign(msg, TestPrivateKey)

	require.False(Verify(difMsg, TestPrivateKey.PublicKey(), sig),
		"Verify incorrectly verified a message")
}

func BenchmarkStdLibVerifySingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		msg := make([]byte, 128)
		_, err := rand.Read(msg)
		if err != nil {
			panic(err)
		}
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}
		sig := ed25519.Sign(priv, msg)
		b.StartTimer()
		if !ed25519.Verify(pub, msg, sig) {
			panic("invalid signature")
		}
	}
}

func BenchmarkOasisVerifySingle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		msg := make([]byte, 128)
		_, err := rand.Read(msg)
		if err != nil {
			panic(err)
		}
		pub, priv, err := oed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}
		sig := oed25519.Sign(priv, msg)
		b.StartTimer()
		if !oed25519.VerifyWithOptions(pub, msg, sig, &verifyOptions) {
			panic("invalid signature")
		}
	}
}

func BenchmarkOasisVerifyCache(b *testing.B) {
	cacheVerifier := cache.NewVerifier(cache.NewLRUCache(10000))
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		msg := make([]byte, 128)
		_, err := rand.Read(msg)
		if err != nil {
			panic(err)
		}
		pub, priv, err := oed25519.GenerateKey(nil)
		if err != nil {
			panic(err)
		}
		sig := oed25519.Sign(priv, msg)
		cacheVerifier.AddPublicKey(pub)
		b.StartTimer()
		if !cacheVerifier.VerifyWithOptions(pub, msg, sig, &verifyOptions) {
			panic("invalid signature")
		}
	}
}

func BenchmarkOasisBatchVerify(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				pubs := make([][]byte, numItems)
				msgs := make([][]byte, numItems)
				sigs := make([][]byte, numItems)
				for j := 0; j < numItems; j++ {
					pub, priv, err := oed25519.GenerateKey(nil)
					if err != nil {
						panic(err)
					}
					pubs[j] = pub
					msg := make([]byte, 128)
					_, err = rand.Read(msg)
					if err != nil {
						panic(err)
					}
					msgs[j] = msg
					sig := oed25519.Sign(priv, msg)
					sigs[j] = sig
				}
				b.StartTimer()
				bv := oed25519.NewBatchVerifierWithCapacity(numItems)
				for j := 0; j < numItems; j++ {
					bv.AddWithOptions(pubs[j], msgs[j], sigs[j], &verifyOptions)
				}
				if !bv.VerifyBatchOnly(nil) {
					panic("invalid signature")
				}
			}
		})
	}
}

func BenchmarkOasisBatchVerifyCache(b *testing.B) {
	cacheVerifier := cache.NewVerifier(cache.NewLRUCache(30000))
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				pubs := make([][]byte, numItems)
				msgs := make([][]byte, numItems)
				sigs := make([][]byte, numItems)
				for j := 0; j < numItems; j++ {
					pub, priv, err := oed25519.GenerateKey(nil)
					if err != nil {
						panic(err)
					}
					cacheVerifier.AddPublicKey(pub)
					pubs[j] = pub
					msg := make([]byte, 128)
					_, err = rand.Read(msg)
					if err != nil {
						panic(err)
					}
					msgs[j] = msg
					sig := oed25519.Sign(priv, msg)
					sigs[j] = sig
				}
				b.StartTimer()
				bv := oed25519.NewBatchVerifierWithCapacity(numItems)
				for j := 0; j < numItems; j++ {
					cacheVerifier.AddWithOptions(bv, pubs[j], msgs[j], sigs[j], &verifyOptions)
				}
				if !bv.VerifyBatchOnly(nil) {
					panic("invalid signature")
				}
			}
		})
	}
}
