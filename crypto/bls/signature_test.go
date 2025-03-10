// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"
)

func TestSignatureBytes(t *testing.T) {
	require := require.New(t)

	msg := utils.RandomBytes(1234)

	sk, err := GeneratePrivateKey()
	require.NoError(err)
	sig, err := Sign(msg, sk)
	require.NoError(err)
	sigBytes := SignatureToBytes(sig)

	sig2, err := SignatureFromBytes(sigBytes)
	require.NoError(err)
	sig2Bytes := SignatureToBytes(sig2)

	require.Equal(sig, sig2)
	require.Equal(sigBytes, sig2Bytes)
}

func TestAggregateSignaturesNoop(t *testing.T) {
	require := require.New(t)

	msg := utils.RandomBytes(1234)

	sk, err := GeneratePrivateKey()
	require.NoError(err)

	sig, err := Sign(msg, sk)
	require.NoError(err)
	sigBytes := SignatureToBytes(sig)

	aggSig, err := AggregateSignatures([]*Signature{sig})
	require.NoError(err)

	aggSigBytes := SignatureToBytes(aggSig)
	require.NoError(err)

	require.Equal(sig, aggSig)
	require.Equal(sigBytes, aggSigBytes)
}
