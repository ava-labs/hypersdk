// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/json"
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
		ActionWithOutput{},
		FixedBytes{},
		Bools{},
	}, []codec.Typed{
		ActionOutput{},
	})
	require.NoError(err)

	expectedABIJSON := mustReadFile(t, "testdata/abi.json")
	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

	require.Equal(expectedABI, actualABI)
}

func TestGetABIofABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]codec.Typed{
		ABI{},
	}, []codec.Typed{})
	require.NoError(err)

	expectedABIJSON := mustReadFile(t, "testdata/abi.abi.json")
	expectedABI := mustJSONParse[ABI](t, string(expectedABIJSON))

	require.Equal(expectedABI, actualABI)
}

func mustJSONParse[T any](t *testing.T, jsonStr string) T {
	var parsed T
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, jsonStr)
	return parsed
}
