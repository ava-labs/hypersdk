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

func TestAddressStringIdentity(t *testing.T) {
	require := require.New(t)
	typeID := byte(0)
	addrID := ids.GenerateTestID()

	addr := CreateAddress(typeID, addrID)

	originalAddr, err := StringToAddress(addr.String())
	require.NoError(err)
	require.Equal(addr, originalAddr)
}

func TestZeroAddressToString(t *testing.T) {
	require := require.New(t)
	expectedStr := "0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9"
	require.Equal(expectedStr, EmptyAddress.String())
}

func TestAddressToString(t *testing.T) {
	require := require.New(t)
	expectedStr := "0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20a10df6ab"
	addr := CreateAddress(
		byte(0),
		[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
	)
	require.Equal(expectedStr, addr.String())
}

func TestStringToAddress(t *testing.T) {
	addr := CreateAddress(
		byte(0),
		[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
	)
	addrStr := "0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20a10df6ab"

	// Remove checksum from address string
	addrStrWithNoChecksum := addrStr[:len(addrStr)-4]
	// Manipulate last character of address string
	addrStrWithBadChecksum := addrStr[:len(addrStr)-1] + "c"

	tests := []struct {
		name            string
		inputStr        string
		expectedAddress Address
		expectedErr     error
	}{
		{
			name:            "simpleChecksummedAddress",
			inputStr:        addrStr,
			expectedAddress: addr,
			expectedErr:     nil,
		},
		{
			name:            "simpleChecksummedAddressWithNoPrefix",
			inputStr:        addrStr[2:],
			expectedAddress: addr,
			expectedErr:     nil,
		},
		{
			name:            "addressWithNoChecksum",
			inputStr:        addrStrWithNoChecksum,
			expectedAddress: EmptyAddress,
			expectedErr:     ErrBadChecksum,
		},
		{
			name:            "addressLengthSmallerThanChecksum",
			inputStr:        "0x0000",
			expectedAddress: EmptyAddress,
			expectedErr:     ErrMissingChecksum,
		},
		{
			name:            "addressWithBadChecksum",
			inputStr:        addrStrWithBadChecksum,
			expectedAddress: EmptyAddress,
			expectedErr:     ErrBadChecksum,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)
			addr, err := StringToAddress(test.inputStr)
			require.Equal(test.expectedAddress, addr)
			require.Equal(test.expectedErr, err)
		})
	}
}
