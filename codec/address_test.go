// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"bytes"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

const hrp = "blah"

func TestIDAddress(t *testing.T) {
	require := require.New(t)

	id := ids.GenerateTestID()
	addrBytes := CreateAddress(0, id)
	addr, err := AddressBech32(hrp, addrBytes)
	require.NoError(err)

	sb, err := ParseAddressBech32(hrp, addr)
	require.NoError(err)
	require.True(bytes.Equal(addrBytes[:], sb[:]))
}

func TestInvalidAddressHRP(t *testing.T) {
	require := require.New(t)
	addr := "blah1859dz2uwazfgahey3j53ef2kqrans0c8cv4l78tda3rjkfw0txns8u2e8k"

	_, err := ParseAddressBech32("test", addr)
	require.ErrorIs(err, ErrIncorrectHRP)
}

func TestInvalidAddressChecksum(t *testing.T) {
	require := require.New(t)
	addr := "blah1859dz2uwazfgahey3j53ef2kqrans0c8cv4l78tda3rjkfw0txns8u2e7k"

	_, err := ParseAddressBech32(hrp, addr)
	require.ErrorContains(err, "invalid checksum")
}
