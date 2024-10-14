// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"encoding/hex"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/stretchr/testify/require"
)

const (
	expectedUnsignedWarpCertificateHex = "00000001869f3d0ad12b8ee8928edf248ca91ca55600fb383f07c32bff1d6dec472b25cf59a700000038000000000001000000000000002a0000e902a9a86640bfdb1cd0e36c0cc982b83e5765fad5f6bbe6abdcce7b5ae7d7c70000000000000049"
	expectedSignedWarpCertHex          = "00000001869f3d0ad12b8ee8928edf248ca91ca55600fb383f07c32bff1d6dec472b25cf59a700000038000000000001000000000000002a0000e902a9a86640bfdb1cd0e36c0cc982b83e5765fad5f6bbe6abdcce7b5ae7d7c700000000000000490000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
)

func TestUnsignedWarpCertificate(t *testing.T) {
	require := require.New(t)
	var (
		networkID = uint32(99999)
		chainID   = ids.FromStringOrPanic("TtF4d2QWbk5vzQGTEPrN48x6vwgAoAmKQ9cbp79inpQmcRKES")
		chunkID   = ids.FromStringOrPanic("2mcwQKiD8VEspmMJpL1dc7okQQ5dDVAWeCBZ7FWBFAbxpv3t7w")
		slot      = int64(73)
	)

	chunkCert, err := NewUnsignedWarpChunkCertificate(networkID, chainID, chunkID, slot)
	require.NoError(err)
	require.Equal(networkID, chunkCert.UnsignedMessage.NetworkID)
	require.Equal(chainID, chunkCert.UnsignedMessage.SourceChainID)
	require.Equal(chunkID, chunkCert.Payload.ChunkID)
	require.Equal(slot, chunkCert.Payload.Slot)

	chunkCertBytes := chunkCert.Bytes()
	require.Equal(expectedUnsignedWarpCertificateHex, hex.EncodeToString(chunkCertBytes))
	parsedChunkCert, err := ParseUnsignedWarpChunkCertificate(chunkCertBytes)
	require.NoError(err)
	require.Equal(networkID, parsedChunkCert.UnsignedMessage.NetworkID)
	require.Equal(chainID, parsedChunkCert.UnsignedMessage.SourceChainID)
	require.Equal(chunkID, parsedChunkCert.Payload.ChunkID)
	require.Equal(slot, parsedChunkCert.Payload.Slot)
}

func TestWarpCertificate(t *testing.T) {
	require := require.New(t)
	var (
		networkID = uint32(99999)
		chainID   = ids.FromStringOrPanic("TtF4d2QWbk5vzQGTEPrN48x6vwgAoAmKQ9cbp79inpQmcRKES")
		chunkID   = ids.FromStringOrPanic("2mcwQKiD8VEspmMJpL1dc7okQQ5dDVAWeCBZ7FWBFAbxpv3t7w")
		slot      = int64(73)
	)

	chunkCert, err := NewUnsignedWarpChunkCertificate(networkID, chainID, chunkID, slot)
	require.NoError(err)

	signers := set.NewBits().Bytes()
	signature := &warp.BitSetSignature{
		Signers: signers,
	}
	expectedNumSigners, err := signature.NumSigners()
	require.NoError(err)
	signedChunkCert, err := NewWarpChunkCertificate(chunkCert, signature)
	require.NoError(err)

	signedChunkCertBytes := signedChunkCert.Bytes()
	require.Equal(expectedSignedWarpCertHex, hex.EncodeToString(signedChunkCertBytes))
	require.Equal(networkID, signedChunkCert.UnsignedCertificate.UnsignedMessage.NetworkID)
	require.Equal(chainID, signedChunkCert.UnsignedCertificate.UnsignedMessage.SourceChainID)
	require.Equal(chunkID, signedChunkCert.UnsignedCertificate.Payload.ChunkID)
	require.Equal(slot, signedChunkCert.UnsignedCertificate.Payload.Slot)
	numSigners, err := signedChunkCert.Message.Signature.NumSigners()
	require.NoError(err)
	require.Equal(expectedNumSigners, numSigners)

	parsedSignedChunkCert, err := ParseWarpChunkCertificate(signedChunkCertBytes)
	require.NoError(err)
	require.Equal(networkID, parsedSignedChunkCert.UnsignedCertificate.UnsignedMessage.NetworkID)
	require.Equal(chainID, parsedSignedChunkCert.UnsignedCertificate.UnsignedMessage.SourceChainID)
	require.Equal(chunkID, parsedSignedChunkCert.UnsignedCertificate.Payload.ChunkID)
	require.Equal(slot, parsedSignedChunkCert.UnsignedCertificate.Payload.Slot)
	require.Equal(parsedSignedChunkCert.Message.Signature, signature)
}
