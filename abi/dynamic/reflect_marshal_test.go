// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dynamic

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/abi"
)

func TestDynamicMarshal(t *testing.T) {
	require := require.New(t)

	abiJSON := mustReadFile(t, "../testdata/abi.json")
	var abi abi.ABI

	err := json.Unmarshal(abiJSON, &abi)
	require.NoError(err)

	testCases := []struct {
		name     string
		typeName string
	}{
		{"empty", "MockObjectSingleNumber"},
		{"uint16", "MockObjectSingleNumber"},
		{"numbers", "MockObjectAllNumbers"},
		{"arrays", "MockObjectArrays"},
		{"transfer", "MockActionTransfer"},
		{"transferField", "MockActionWithTransfer"},
		{"transfersArray", "MockActionWithTransferArray"},
		{"strBytes", "MockObjectStringAndBytes"},
		{"strByteZero", "MockObjectStringAndBytes"},
		{"strBytesEmpty", "MockObjectStringAndBytes"},
		{"strOnly", "MockObjectStringAndBytes"},
		{"outer", "Outer"},
		{"fixedBytes", "FixedBytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read the JSON data
			jsonData := mustReadFile(t, "../testdata/"+tc.name+".json")

			objectBytes, err := Marshal(abi, tc.typeName, string(jsonData))
			require.NoError(err)

			// Compare with expected hex
			expectedTypeID, found := abi.FindActionByName(tc.typeName)
			require.True(found, "action %s not found in ABI", tc.typeName)

			expectedHex := hex.EncodeToString([]byte{expectedTypeID.ID}) + strings.TrimSpace(string(mustReadFile(t, "../testdata/"+tc.name+".hex")))
			require.Equal(expectedHex, hex.EncodeToString(objectBytes))

			unmarshaledJSON, err := UnmarshalAction(abi, objectBytes)
			require.NoError(err)

			// Compare with expected JSON
			require.JSONEq(string(jsonData), unmarshaledJSON)
		})
	}
}

func TestDynamicMarshalErrors(t *testing.T) {
	require := require.New(t)

	abiJSON := mustReadFile(t, "../testdata/abi.json")
	var abi abi.ABI

	err := json.Unmarshal(abiJSON, &abi)
	require.NoError(err)

	// Test malformed JSON
	malformedJSON := `{"uint8": 42, "uint16": 1000, "uint32": 100000, "uint64": 10000000000, "int8": -42, "int16": -1000, "int32": -100000, "int64": -10000000000,`
	_, err = Marshal(abi, "MockObjectAllNumbers", malformedJSON)
	require.Contains(err.Error(), "unexpected end of JSON input")

	// Test wrong struct name
	jsonData := mustReadFile(t, "../testdata/numbers.json")
	_, err = Marshal(abi, "NonExistentObject", string(jsonData))
	require.ErrorIs(err, ErrTypeNotFound)
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
