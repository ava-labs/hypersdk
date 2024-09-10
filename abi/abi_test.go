// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

func TestABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := GetVMABI([]codec.Typed{
		MockObjectSingleNumber{},
		MockActionTransfer{},
		MockObjectAllNumbers{},
		MockObjectStringAndBytes{},
		MockObjectArrays{},
		MockActionWithTransferArray{},
		MockActionWithTransfer{},
		Outer{},
	})
	require.NoError(err)

	expectedAbiJSON := mustReadFile(t, "testdata/abi.json")
	expectedAbi := mustJSONParse[VMABI](t, string(expectedAbiJSON))

	require.Equal(expectedAbi, actualABI)
}

func TestABIsABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := GetVMABI([]codec.Typed{VMABI{}})
	require.NoError(err)

	expectedAbiJSON := mustReadFile(t, "testdata/abi.abi.json")
	expectedAbi := mustJSONParse[VMABI](t, string(expectedAbiJSON))

	require.Equal(expectedAbi, actualABI)
}
