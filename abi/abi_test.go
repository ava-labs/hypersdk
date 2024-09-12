// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestNewABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]codec.Typed{
		MockObjectSingleNumber{},
		MockActionTransfer{},
		MockObjectAllNumbers{},
		MockObjectStringAndBytes{},
		MockObjectArrays{},
		MockActionWithTransfer{},
		MockActionWithTransferArray{},
		Outer{},
	})
	require.NoError(err)

	expectedABIJSON := mustReadFile(t, "testdata/abi.json")
	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

	require.Equal(expectedABI, actualABI)
}

func TestGetABIofABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]codec.Typed{ABI{}})
	require.NoError(err)

	expectedABIJSON := mustReadFile(t, "testdata/abi.abi.json")
	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

	require.Equal(expectedABI, actualABI)
}
