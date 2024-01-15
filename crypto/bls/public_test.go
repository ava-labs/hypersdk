// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bls

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
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
