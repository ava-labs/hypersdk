package secp256k1_test

import (
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/crypto/secp256k1"
)

func TestGeneratePrivateKey(t *testing.T) {
	require := require.New(t)
	priv, err := secp256k1.GeneratePrivateKey()
	require.NoError(err)
	require.Len(priv, secp256k1.PrivateKeyLen)
}

func TestGeneratePrivateKeyDifferent(t *testing.T) {
	require := require.New(t)
	const numKeysToGenerate int = 10
	pks := [numKeysToGenerate]secp256k1.PrivateKey{}

	// generate keys
	for i := 0; i < numKeysToGenerate; i++ {
		priv, err := secp256k1.GeneratePrivateKey()
		pks[i] = priv
		require.NoError(err, "Error Generating Private Key")
	}

	// make sure keys are different
	m := make(map[secp256k1.PrivateKey]bool)
	for _, priv := range pks {
		require.False(m[priv], "Duplicate PrivateKey generated")
		m[priv] = true
	}
}

func TestPublicKeyFormat(t *testing.T) {
	require := require.New(t)
	priv, err := secp256k1.GeneratePrivateKey()
	require.NoError(err)
	pubKey := priv.PublicKey()
	require.NotEqual(pubKey, secp256k1.EmptyPublicKey)
	require.Len(pubKey, secp256k1.PublicKeyLen)
}

func TestSignVerify(t *testing.T) {
	require := require.New(t)
	for i := 0; i < 1000; i++ {
		// Generate private keys
		priv, err := secp256k1.GeneratePrivateKey()
		require.NoError(err)

		// Sign message
		msg := []byte("hello")
		sig := priv.Sign(msg)

		// Verify signature
		require.True(secp256k1.Verify(msg, priv.PublicKey(), sig))
	}
}

func TestInvalidSignature(t *testing.T) {
	require := require.New(t)
	msg := []byte("hello")
	priv, err := secp256k1.GeneratePrivateKey()
	require.NoError(err)

	// Generate valid signature and modify it
	sig := priv.Sign(msg)
	sig[0]++ // corrupt the signatur

	require.False(secp256k1.Verify(msg, priv.PublicKey(), sig))
}

func TestEmptySignature(t *testing.T) {
	require := require.New(t)
	msg := []byte("hello")
	priv, err := secp256k1.GeneratePrivateKey()
	require.NoError(err)

	require.False(secp256k1.Verify(msg, priv.PublicKey(), secp256k1.EmptySignature))
}

func TestEmptyPublicKey(t *testing.T) {
	require := require.New(t)
	msg := []byte("hello")
	priv, err := secp256k1.GeneratePrivateKey()
	require.NoError(err)
	sig := priv.Sign(msg)

	require.False(secp256k1.Verify(msg, secp256k1.EmptyPublicKey, sig))
}

func BenchmarkSignVerify(b *testing.B) {
	for _, numItems := range []int{1, 4, 16, 64, 128, 512, 1024, 4096, 16384} {
		b.Run(strconv.Itoa(numItems), func(b *testing.B) {
			require := require.New(b)
			b.StopTimer()
			pubs := make([]secp256k1.PublicKey, numItems)
			msgs := make([][]byte, numItems)
			sigs := make([]secp256k1.Signature, numItems)
			for i := 0; i < numItems; i++ {
				priv, err := secp256k1.GeneratePrivateKey()
				require.NoError(err)
				pubs[i] = priv.PublicKey()
				msg := make([]byte, 128)
				_, err = rand.Read(msg)
				require.NoError(err)
				msgs[i] = msg
				sigs[i] = priv.Sign(msg)
			}
			b.StartTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for j := 0; j < numItems; j++ {
					require.True(secp256k1.Verify(msgs[j], pubs[j], sigs[j]))
				}
			}
		})
	}
}
