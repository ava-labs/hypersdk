// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// Sets the expected ABI hash for the testdata/abi.json file
// Used to verify implementation in other languages
func TestABIHash(t *testing.T) {
	require := require.New(t)

	// get spec from file
	abiJSON := mustReadFile(t, "testdata/abi.json")
	var abiFromFile ABI
	err := json.Unmarshal(abiJSON, &abiFromFile)
	require.NoError(err)

	// check hash and compare it to expected
	abiBytes := chain.MustMarshal(&abiFromFile)

	abiHash := sha256.Sum256(abiBytes)
	expectedHashHex := strings.TrimSpace(string(mustReadFile(t, "testdata/abi.hash.hex")))
	require.Equal(expectedHashHex, hex.EncodeToString(abiHash[:]))
}

// Used to verify implementation in other languages, relies on testdata dir
func TestMarshalSpecs(t *testing.T) {
	require := require.New(t)

	testCases := []struct {
		name   string
		object codec.Typed
	}{
		{"empty", &MockObjectSingleNumber{}},
		{"uint16", &MockObjectSingleNumber{}},
		{"numbers", &MockObjectAllNumbers{}},
		{"arrays", &MockObjectArrays{}},
		{"transfer", &MockActionTransfer{}},
		{"transferField", &MockActionWithTransfer{}},
		{"transfersArray", &MockActionWithTransferArray{}},
		{"strBytes", &MockObjectStringAndBytes{}},
		{"strByteZero", &MockObjectStringAndBytes{}},
		{"strBytesEmpty", &MockObjectStringAndBytes{}},
		{"strOnly", &MockObjectStringAndBytes{}},
		{"outer", &Outer{}},
		{"fixedBytes", &FixedBytes{}},
		{"bools", &Bools{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a copy of the original object
			unmarshaledFromJSON := reflect.New(reflect.TypeOf(tc.object).Elem()).Interface().(codec.Typed)
			unmarshaledFromBytes := reflect.New(reflect.TypeOf(tc.object).Elem()).Interface().(codec.Typed)

			// Get object from file
			err := json.Unmarshal(mustReadFile(t, "testdata/"+tc.name+".json"), unmarshaledFromJSON)
			require.NoError(err)

			// Marshal the object
			objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
			err = codec.LinearCodec.MarshalInto(unmarshaledFromJSON, objectPacker.Packer)
			require.NoError(err)

			objectBytes := objectPacker.Bytes()

			// Compare with expected hex
			expectedHex := string(mustReadFile(t, "testdata/"+tc.name+".hex"))
			expectedHex = strings.TrimSpace(expectedHex)
			require.Equal(expectedHex, hex.EncodeToString(objectBytes))

			// Unmarshal the object
			err = codec.LinearCodec.UnmarshalFrom(&wrappers.Packer{Bytes: objectBytes}, unmarshaledFromBytes)
			require.NoError(err)

			// Compare unmarshaled object with the original
			require.Equal(unmarshaledFromJSON, unmarshaledFromBytes)
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
