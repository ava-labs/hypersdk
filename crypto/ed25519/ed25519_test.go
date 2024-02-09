// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ed25519

import (
	"crypto/ed25519"
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/hdevalence/ed25519consensus"
	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519/extra/cache"
	"github.com/stretchr/testify/require"

	oed25519 "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
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
	oed25519options = &oed25519.Options{
		Verify: oed25519.VerifyOptionsZIP_215,
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
	require.Equal(expectedSig, sig, "Signature was incorrect")
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

func TestBatchAddVerifyValid(t *testing.T) {
	require := require.New(t)
	var (
		numItems = 1024
		pubs     = make([]PublicKey, numItems)
		msgs     = make([][]byte, numItems)
		sigs     = make([]Signature, numItems)
	)
	for i := 0; i < numItems; i++ {
		priv, err := GeneratePrivateKey()
		require.NoError(err)
		pubs[i] = priv.PublicKey()
		msg := make([]byte, 128)
		_, err = rand.Read(msg)
		require.NoError(err)
		msgs[i] = msg
		sig := Sign(msg, priv)
		sigs[i] = sig
	}
	bv := NewBatch(numItems)
	for i := 0; i < numItems; i++ {
		bv.Add(msgs[i], pubs[i], sigs[i])
	}
	require.True(bv.Verify(), "invalid signature")
}

func TestBatchAddVerifyInvalid(t *testing.T) {
	require := require.New(t)
	var (
		numItems = 1024
		pubs     = make([]PublicKey, numItems)
		msgs     = make([][]byte, numItems)
		sigs     = make([]Signature, numItems)
	)
	for i := 0; i < numItems; i++ {
		priv, err := GeneratePrivateKey()
		require.NoError(err)
		pubs[i] = priv.PublicKey()
		msg := make([]byte, 128)
		_, err = rand.Read(msg)
		require.NoError(err)
		msgs[i] = msg
		sig := Sign(msg, priv)
		if i == 10 {
			sig[0]++
		}
		sigs[i] = sig
	}
	bv := NewBatch(numItems)
	for i := 0; i < numItems; i++ {
		bv.Add(msgs[i], pubs[i], sigs[i])
	}
	require.False(bv.Verify(), "valid signature")
}

func BenchmarkStdLibVerifySingle(b *testing.B) {
	require := require.New(b)
	b.StopTimer()
	msg := make([]byte, 128)
	_, err := rand.Read(msg)
	require.NoError(err)
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(err)
	sig := ed25519.Sign(priv, msg)
	b.StartTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		require.True(ed25519.Verify(pub, msg, sig), "invalid signature")
	}
}

func BenchmarkConsensusVerifySingle(b *testing.B) {
	require := require.New(b)
	b.StopTimer()
	msg := make([]byte, 128)
	_, err := rand.Read(msg)
	require.NoError(err)
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(err)
	sig := ed25519.Sign(priv, msg)
	b.StartTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		require.True(ed25519consensus.Verify(pub, msg, sig), "invalid signature")
	}
}

func BenchmarkOasisVerifySingle(b *testing.B) {
	require := require.New(b)
	b.StopTimer()
	msg := make([]byte, 128)
	_, err := rand.Read(msg)
	require.NoError(err)
	pub, priv, err := oed25519.GenerateKey(nil)
	require.NoError(err)
	sig := oed25519.Sign(priv, msg)
	b.StartTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		require.True(oed25519.VerifyWithOptions(pub, msg, sig, oed25519options), "invalid signature")
	}
}

func BenchmarkOasisVerifyCache(b *testing.B) {
	require := require.New(b)
	b.StopTimer()
	cacheVerifier := cache.NewVerifier(cache.NewLRUCache(10000))
	msg := make([]byte, 128)
	_, err := rand.Read(msg)
	require.NoError(err)
	pub, priv, err := oed25519.GenerateKey(nil)
	require.NoError(err)
	sig := oed25519.Sign(priv, msg)
	cacheVerifier.AddPublicKey(pub)
	b.StartTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		require.True(cacheVerifier.VerifyWithOptions(pub, msg, sig, oed25519options), "invalid signature")
	}
}

func BenchmarkConsensusBatchAddVerify(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			pubs := make([][]byte, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([][]byte, numItems)
			for i := 0; i < numItems; i++ {
				pub, priv, err := ed25519.GenerateKey(nil)
				require.NoError(err)
				pubs[i] = pub[:]
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sig := ed25519.Sign(priv, msg)
				sigs[i] = sig
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				bv := ed25519consensus.NewPreallocatedBatchVerifier(numItems)
				for j := 0; j < numItems; j++ {
					bv.Add(pubs[j], msgs[j], sigs[j])
				}
				require.True(bv.Verify(), "invalid signature")
			}
		})
	}
}

func BenchmarkConsensusBatchVerify(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			pubs := make([][]byte, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([][]byte, numItems)
			for i := 0; i < numItems; i++ {
				pub, priv, err := ed25519.GenerateKey(nil)
				require.NoError(err)
				pubs[i] = pub[:]
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sig := ed25519.Sign(priv, msg)
				sigs[i] = sig
			}
			bv := ed25519consensus.NewPreallocatedBatchVerifier(numItems)
			for i := 0; i < numItems; i++ {
				bv.Add(pubs[i], msgs[i], sigs[i])
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				require.True(bv.Verify(), "invalid signature")
			}
		})
	}
}

func BenchmarkOasisBatchAddVerify(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			pubs := make([][]byte, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([][]byte, numItems)
			for i := 0; i < numItems; i++ {
				pub, priv, err := oed25519.GenerateKey(nil)
				require.NoError(err)
				pubs[i] = pub
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sig := oed25519.Sign(priv, msg)
				sigs[i] = sig
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				bv := oed25519.NewBatchVerifierWithCapacity(numItems)
				for j := 0; j < numItems; j++ {
					bv.AddWithOptions(pubs[j], msgs[j], sigs[j], oed25519options)
				}
				require.True(bv.VerifyBatchOnly(nil), "invalid signature")
			}
		})
	}
}

func BenchmarkOasisBatchVerify(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			pubs := make([][]byte, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([][]byte, numItems)
			for i := 0; i < numItems; i++ {
				pub, priv, err := oed25519.GenerateKey(nil)
				require.NoError(err)
				pubs[i] = pub
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sig := oed25519.Sign(priv, msg)
				sigs[i] = sig
			}
			bv := oed25519.NewBatchVerifierWithCapacity(numItems)
			for i := 0; i < numItems; i++ {
				bv.AddWithOptions(pubs[i], msgs[i], sigs[i], oed25519options)
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				require.True(bv.VerifyBatchOnly(nil), "invalid signature")
			}
		})
	}
}

func BenchmarkOasisBatchAddVerifyCache(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			cacheVerifier := cache.NewVerifier(cache.NewLRUCache(30000))
			pubs := make([][]byte, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([][]byte, numItems)
			for i := 0; i < numItems; i++ {
				pub, priv, err := oed25519.GenerateKey(nil)
				require.NoError(err)
				cacheVerifier.AddPublicKey(pub)
				pubs[i] = pub
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sig := oed25519.Sign(priv, msg)
				sigs[i] = sig
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				bv := oed25519.NewBatchVerifierWithCapacity(numItems)
				for j := 0; j < numItems; j++ {
					cacheVerifier.AddWithOptions(bv, pubs[j], msgs[j], sigs[j], oed25519options)
				}
				require.True(bv.VerifyBatchOnly(nil), "invalid signature")
			}
		})
	}
}

func BenchmarkOasisBatchVerifyCache(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			cacheVerifier := cache.NewVerifier(cache.NewLRUCache(30000))
			pubs := make([][]byte, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([][]byte, numItems)
			for i := 0; i < numItems; i++ {
				pub, priv, err := oed25519.GenerateKey(nil)
				require.NoError(err)
				cacheVerifier.AddPublicKey(pub)
				pubs[i] = pub
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sig := oed25519.Sign(priv, msg)
				sigs[i] = sig
			}
			bv := oed25519.NewBatchVerifierWithCapacity(numItems)
			for i := 0; i < numItems; i++ {
				cacheVerifier.AddWithOptions(bv, pubs[i], msgs[i], sigs[i], oed25519options)
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				require.True(bv.VerifyBatchOnly(nil), "invalid signature")
			}
		})
	}
}
