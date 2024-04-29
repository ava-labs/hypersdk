// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/stretchr/testify/require"
)

func TestPrivateKeyFromBytesZero(t *testing.T) {
	require := require.New(t)

	var skArr [PrivateKeyLen]byte
	skBytes := skArr[:]
	_, err := PrivateKeyFromBytes(skBytes)
	require.ErrorIs(err, errFailedPrivateKeyDeserialize)
}

func TestPrivateKeyFromBytesWrongSize(t *testing.T) {
	require := require.New(t)

	skBytes := utils.RandomBytes(PrivateKeyLen + 1)
	_, err := PrivateKeyFromBytes(skBytes)
	require.ErrorIs(err, errFailedPrivateKeyDeserialize)
}

func TestPrivateKeyBytes(t *testing.T) {
	require := require.New(t)

	msg := utils.RandomBytes(1234)

	sk, err := GeneratePrivateKey()
	require.NoError(err)
	sig := Sign(msg, sk)
	skBytes := PrivateKeyToBytes(sk)

	sk2, err := PrivateKeyFromBytes(skBytes)
	require.NoError(err)
	sig2 := Sign(msg, sk2)
	sk2Bytes := PrivateKeyToBytes(sk2)

	require.Equal(sk, sk2)
	require.Equal(skBytes, sk2Bytes)
	require.Equal(sig, sig2)
}
