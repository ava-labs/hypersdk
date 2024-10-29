// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func TestAddress(t *testing.T) {
	require := require.New(t)
	typeID := byte(0)
	addrID := ids.GenerateTestID()

	addr := CreateAddress(typeID, addrID)
	addrStr, err := addr.MarshalText()
	require.NoError(err)

	var parsedAddr Address
	require.NoError(parsedAddr.UnmarshalText(addrStr))
	require.Equal(addr, parsedAddr)
}

func TestAddressJSON(t *testing.T) {
	require := require.New(t)
	typeID := byte(0)
	addrID := ids.GenerateTestID()

	addr := CreateAddress(typeID, addrID)

	addrJSONBytes, err := json.Marshal(addr)
	require.NoError(err)

	var parsedAddr Address
	require.NoError(json.Unmarshal(addrJSONBytes, &parsedAddr))
	require.Equal(addr, parsedAddr)
}

func TestEncodeWithChecksum(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		address     Address
		checksumStr string
	}{
		{
			address: CreateAddress(
				byte(0),
				ids.Empty,
			),
			checksumStr: "0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9",
		},
		{
			address: CreateAddress(
				byte(0),
				[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			),
			checksumStr: "0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20a10df6ab",
		},
		{
			address: CreateAddress(
				byte(16),
				[32]byte{33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64},
			),
			checksumStr: "0x102122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f40182496b1",
		},
	}

	for _, test := range tests {
		require.Equal(test.checksumStr, encodeWithChecksum(test.address[:]))
	}
}

func TestFromChecksum(t *testing.T) {
	require := require.New(t)
	zeroAddress := CreateAddress(byte(0), ids.Empty)

	tests := []struct {
		name          string
		inputStr      string
		expectedBytes []byte
		expectedErr   error
	}{
		{
			name:          "simpleChecksummedAddress",
			inputStr:      "0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9",
			expectedBytes: zeroAddress[:],
			expectedErr:   nil,
		},
		{
			name:          "addressWithNoChecksum",
			inputStr:      "0x000000000000000000000000000000000000000000000000000000000000000000",
			expectedBytes: nil,
			expectedErr:   ErrBadChecksum,
		},
		{
			name:          "addressWithBadChecksum",
			inputStr:      "0x000000000000000000000000000000000000000000000000000000000000000000b7396ce9",
			expectedBytes: nil,
			expectedErr:   ErrBadChecksum,
		},
	}

	for _, test := range tests {
		originalBytes, err := fromChecksum(test.inputStr)
		require.Equal(test.expectedBytes, originalBytes)
		require.Equal(test.expectedErr, err)
	}
}
