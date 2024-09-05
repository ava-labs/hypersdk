// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	_ "embed"

	"github.com/ava-labs/hypersdk/abi/testdata"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// Combined VMABI and AutoMarshal spec
// Used to verify TypeScript implementation
// Tests added as needed by TypeScript
// Ensures consistency in marshaling, not testing Go struct marshaling itself

func TestABIHash(t *testing.T) {
	require := require.New(t)

	//get spec from file
	abiJSON := mustReadFile(t, "testdata/abi.json")
	var abiFromFile VMABI
	err := json.Unmarshal(abiJSON, &abiFromFile)
	require.NoError(err)

	//check hash and compare it to expected
	abiHash := abiFromFile.Hash()
	expectedHashHex := string(mustReadFile(t, "testdata/abi.hash.hex"))
	require.Equal(expectedHashHex, hex.EncodeToString(abiHash[:]))
}

func TestMarshalSpecs(t *testing.T) {
	require := require.New(t)

	testCases := []struct {
		name   string
		object codec.Typed
	}{
		{"empty", &testdata.MockObjectSingleNumber{}},
		{"uint16", &testdata.MockObjectSingleNumber{}},
		{"numbers", &testdata.MockObjectAllNumbers{}},
		{"arrays", &testdata.MockObjectArrays{}},
		{"transfer", &testdata.MockActionTransfer{}},
		{"transferField", &testdata.MockActionWithTransfer{}},
		{"transfersArray", &testdata.MockActionWithTransferArray{}},
		{"strBytes", &testdata.MockObjectStringAndBytes{}},
		{"strByteZero", &testdata.MockObjectStringAndBytes{}},
		{"strBytesEmpty", &testdata.MockObjectStringAndBytes{}},
		{"strOnly", &testdata.MockObjectStringAndBytes{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get object from file
			err := json.Unmarshal(mustReadFile(t, "testdata/"+tc.name+".json"), tc.object)
			require.NoError(err)

			// Get spec from file
			abiJSON := mustReadFile(t, "testdata/abi.json")
			var abiFromFile VMABI
			err = json.Unmarshal(abiJSON, &abiFromFile)
			require.NoError(err)

			// Marshal the object
			objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
			err = codec.LinearCodec.MarshalInto(tc.object, objectPacker.Packer)
			require.NoError(err)

			objectDigest := objectPacker.Bytes()

			// Compare with expected hex
			expectedHex := string(mustReadFile(t, "testdata/"+tc.name+".hex"))
			expectedHex = strings.TrimSpace(expectedHex)
			require.Equal(expectedHex, hex.EncodeToString(objectDigest), tc.name)
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
