// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/stretchr/testify/require"
)

func TestPublicKeyFromBytesWrongSize(t *testing.T) {
	require := require.New(t)

	pkBytes := utils.RandomBytes(PublicKeyLen + 1)
	_, err := PublicKeyFromBytes(pkBytes)
	require.ErrorIs(err, bls.ErrFailedPublicKeyDecompress)
}

func TestPublicKeyBytes(t *testing.T) {
	require := require.New(t)

	sk, err := GeneratePrivateKey()
	require.NoError(err)

	pk := PublicFromPrivateKey(sk)
	pkBytes := PublicKeyToBytes(pk)

	pk2, err := PublicKeyFromBytes(pkBytes)
	require.NoError(err)
	pk2Bytes := PublicKeyToBytes(pk2)

	require.Equal(pk, pk2)
	require.Equal(pkBytes, pk2Bytes)
}

func TestAggregatePublicKeysNoop(t *testing.T) {
	require := require.New(t)

	sk, err := GeneratePrivateKey()
	require.NoError(err)

	pk := PublicFromPrivateKey(sk)
	pkBytes := PublicKeyToBytes(pk)

	aggPK, err := AggregatePublicKeys([]*PublicKey{pk})
	require.NoError(err)

	aggPKBytes := PublicKeyToBytes(aggPK)
	require.NoError(err)

	require.Equal(pk, aggPK)
	require.Equal(pkBytes, aggPKBytes)
}

func TestAggregatePublicKeys(t *testing.T) {
	require := require.New(t)

	msg := utils.RandomBytes(1234)

	// Public Key 1
	sk1, err := GeneratePrivateKey()
	require.NoError(err)
	pk1 := PublicFromPrivateKey(sk1)

	// Public Key 2
	sk2, err := GeneratePrivateKey()
	require.NoError(err)
	pk2 := PublicFromPrivateKey(sk2)

	// Aggregates 2 public keys
	aggPks, err := AggregatePublicKeys([]*PublicKey{pk1, pk2})
	require.NoError(err)

	// Aggregates 2 signatures
	sig1 := Sign(msg, sk1)
	sig2 := Sign(msg, sk2)
	aggSigs, err := AggregateSignatures([]*Signature{sig1, sig2})
	require.NoError(err)

	// Verifies aggregate signature with aggregate public key
	require.True(Verify(msg, aggPks, aggSigs))
}
