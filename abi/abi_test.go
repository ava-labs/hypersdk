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

	expectedAbiJSON := mustReadFile(t, "testdata/abi.json")
	expectedAbi := mustJSONParse[ABI](t, string(expectedAbiJSON))

	require.Equal(expectedAbi, actualABI)
}

func TestGetABIofABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]codec.Typed{ABI{}})
	require.NoError(err)

	expectedAbiJSON := mustReadFile(t, "testdata/abi.abi.json")
	expectedAbi := mustJSONParse[ABI](t, string(expectedAbiJSON))

	require.Equal(expectedAbi, actualABI)
}
