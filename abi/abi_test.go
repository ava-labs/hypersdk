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
	})
	require.NoError(err)

	expectedAbiJSON := mustReadFile(t, "testdata/abi.json")
	expectedAbi := mustJSONParse[ABI](t, string(expectedAbiJSON))

	require.Equal(mustPrintOrderedJSON(t, expectedAbi), mustPrintOrderedJSON(t, actualABI))
}

func mustPrintOrderedJSON(t *testing.T, v any) string {
	bytes, err := json.Marshal(v)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(bytes, &parsed))

	ordered, err := json.Marshal(parsed)
	require.NoError(t, err)
	return string(ordered)
}

func TestGetABIofABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := NewABI([]codec.Typed{ABI{}})
	require.NoError(err)

	expectedAbiJSON := mustReadFile(t, "testdata/abi.abi.json")
	expectedAbi := mustJSONParse[ABI](t, string(expectedAbiJSON))

	require.Equal(expectedAbi, actualABI)
}
