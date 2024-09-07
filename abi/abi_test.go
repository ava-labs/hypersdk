// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi/testdata"
	"github.com/ava-labs/hypersdk/codec"
)

func TestABI(t *testing.T) {
	require := require.New(t)

	actualABI, err := GetVMABI([]codec.Typed{
		testdata.MockObjectSingleNumber{},
		testdata.MockActionTransfer{},
		testdata.MockObjectAllNumbers{},
		testdata.MockObjectStringAndBytes{},
		testdata.MockObjectArrays{},
		testdata.MockActionWithTransferArray{},
		testdata.MockActionWithTransfer{},
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

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   255,
				Name: "VMABI",
				Types: []SingleTypeABI{
					{
						Name: "VMABI",
						Fields: []ABIField{
							{Name: "actions", Type: "[]SingleActionABI"},
						},
					},
					{
						Name: "SingleActionABI",
						Fields: []ABIField{
							{Name: "id", Type: "uint8"},
							{Name: "name", Type: "string"},
							{Name: "types", Type: "[]SingleTypeABI"},
						},
					},
					{
						Name: "SingleTypeABI",
						Fields: []ABIField{
							{Name: "name", Type: "string"},
							{Name: "fields", Type: "[]ABIField"},
						},
					},
					{
						Name: "ABIField",
						Fields: []ABIField{
							{Name: "name", Type: "string"},
							{Name: "type", Type: "string"},
						},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)

	expectedABIHash := "c92d3b95bd5f73a81568f80ba4ce2e4ad0c54f8ef1f5bc79895411c263b87552"
	actualABIHash := actualABI.Hash()
	require.Equal(expectedABIHash, hex.EncodeToString(actualABIHash[:]))
}
