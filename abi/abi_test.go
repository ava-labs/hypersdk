// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
)

func TestNewABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]chain.ActionPair{
		{Input: MockObjectSingleNumber{}},
		{Input: MockActionTransfer{}},
		{Input: MockObjectAllNumbers{}},
		{Input: MockObjectStringAndBytes{}},
		{Input: MockObjectArrays{}},
		{Input: MockActionWithTransfer{}},
		{Input: MockActionWithTransferArray{}},
		{Input: Outer{}},
		{Input: ActionWithOutput{}, Output: ActionOutput{}},
	})
	require.NoError(err)

	expectedABIJSON := mustReadFile(t, "testdata/abi.json")
	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

	require.Equal(expectedABI, actualABI)
}

func TestGetABIofABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]chain.ActionPair{
		{Input: ABI{}},
	})
	require.NoError(err)

	expectedABIJSON := mustReadFile(t, "testdata/abi.abi.json")
	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

	require.Equal(expectedABI, actualABI)
}
