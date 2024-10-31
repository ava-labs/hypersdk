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

func TestAddressString(t *testing.T) {
	require := require.New(t)
	typeID := byte(0)
	addrID := ids.GenerateTestID()

	addr := CreateAddress(typeID, addrID)

	originalAddr, err := StringToAddress(addr.String())
	require.NoError(err)
	require.Equal(addr, originalAddr)
}

func TestEncodeWithChecksum(t *testing.T) {
	t.Run("zeroAddressChecksum", func(t *testing.T) {
		require := require.New(t)
		expectedStr := "0x000000000000000000000000000000000000000000000000000000000000000000a7396ce9"
		require.Equal(expectedStr, EmptyAddress.String())
	})

	t.Run("arbitraryAddressChecksum", func(t *testing.T) {
		require := require.New(t)
		expectedStr := "0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20a10df6ab"
		addr := CreateAddress(
			byte(0),
			[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
		)
		require.Equal(expectedStr, addr.String())
	})
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
