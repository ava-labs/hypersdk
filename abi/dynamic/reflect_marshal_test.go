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

	// Load the ABI
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

			// Use DynamicMarshal to marshal the data
			objectBytes, err := DynamicMarshal(abi, tc.typeName, string(jsonData))
			require.NoError(err)

			// Compare with expected hex
			expectedHex := string(mustReadFile(t, "../testdata/"+tc.name+".hex"))
			expectedHex = strings.TrimSpace(expectedHex)
			require.Equal(expectedHex, hex.EncodeToString(objectBytes))

			// Use DynamicUnmarshal to unmarshal the data
			unmarshaledJSON, err := DynamicUnmarshal(abi, tc.typeName, objectBytes)
			require.NoError(err)

			// Compare with expected JSON
			require.JSONEq(string(jsonData), unmarshaledJSON)
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
