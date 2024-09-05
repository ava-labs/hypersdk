// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	_ "embed"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// Combined VMABI and AutoMarshal spec
// Used to verify TypeScript implementation
// Tests added as needed by TypeScript
// Ensures consistency in marshaling, not testing Go struct marshaling itself

type MockObjectSingleNumber struct {
	Field1 uint16 `serialize:"true"`
}

func (MockObjectSingleNumber) GetTypeID() uint8 {
	return 1
}

type MockActionTransfer struct {
	To    codec.Address       `serialize:"true" json:"to"`
	Value uint64              `serialize:"true" json:"value"`
	Memo  codec.StringAsBytes `serialize:"true" json:"memo"`
}

func (MockActionTransfer) GetTypeID() uint8 {
	return 2
}

func TestABIHash(t *testing.T) {
	require := require.New(t)

	//get spec from file
	abiJSON := mustReadFile(t, "test_data/abi.json")
	var abiFromFile VMABI
	err := json.Unmarshal(abiJSON, &abiFromFile)
	require.NoError(err)

	//TODO: maybe check file generation?

	//check hash and compare it to expected
	abiHash := abiFromFile.Hash()
	expectedHashHex := string(mustReadFile(t, "test_data/abi.hash.hex"))
	require.Equal(expectedHashHex, hex.EncodeToString(abiHash[:]))
}

func TestMarshalEmptySpec(t *testing.T) {
	require := require.New(t)

	//get object from file
	var object1Instance MockObjectSingleNumber
	err := json.Unmarshal(mustReadFile(t, "test_data/empty.data.json"), &object1Instance)
	require.NoError(err)

	//get spec from file
	abiJSON := mustReadFile(t, "test_data/abi.json")
	var abiFromFile VMABI
	err = json.Unmarshal(abiJSON, &abiFromFile)
	require.NoError(err)

	// This is the output of the combination of above JSONs
	objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(object1Instance, objectPacker.Packer)
	require.NoError(err)

	objectDigest := objectPacker.Bytes()

	expectedHex := string(mustReadFile(t, "test_data/empty.data.hex"))
	require.Equal(expectedHex, hex.EncodeToString(objectDigest))
}

func TestMarshalSingleNumberSpec(t *testing.T) {
	require := require.New(t)

	//get object from file
	var object1Instance MockObjectSingleNumber
	err := json.Unmarshal(mustReadFile(t, "test_data/uint16.data.json"), &object1Instance)
	require.NoError(err)

	//get spec from file
	abiJSON := mustReadFile(t, "test_data/abi.json")
	var abiFromFile VMABI
	err = json.Unmarshal(abiJSON, &abiFromFile)
	require.NoError(err)

	// This is the output of the combination of above JSONs
	objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(object1Instance, objectPacker.Packer)
	require.NoError(err)

	objectDigest := objectPacker.Bytes()

	expectedHex := string(mustReadFile(t, "test_data/uint16.data.hex"))
	require.Equal(expectedHex, hex.EncodeToString(objectDigest))
}

type MockObjectAllNumbers struct {
	Uint8  uint8  `serialize:"true" json:"uint8"`
	Uint16 uint16 `serialize:"true" json:"uint16"`
	Uint32 uint32 `serialize:"true" json:"uint32"`
	Uint64 uint64 `serialize:"true" json:"uint64"`
	Int8   int8   `serialize:"true" json:"int8"`
	Int16  int16  `serialize:"true" json:"int16"`
	Int32  int32  `serialize:"true" json:"int32"`
	Int64  int64  `serialize:"true" json:"int64"`
}

func (MockObjectAllNumbers) GetTypeID() uint8 {
	return 3
}

func TestMarshalAllNumbersSpec(t *testing.T) {
	require := require.New(t)

	object := MockObjectAllNumbers{
		Uint8:  254,
		Uint16: 65534,
		Uint32: 4294967294,
		Uint64: 18446744073709551614,
		Int8:   -127,
		Int16:  -32767,
		Int32:  -2147483647,
		Int64:  -9223372036854775807,
	}

	structJSON, err := json.Marshal(object)
	require.NoError(err)

	expectedStructJSON := `
	{
		"uint8": 254,
		"uint16": 65534,
		"uint32": 4294967294,
		"uint64": 18446744073709551614,
		"int8": -127,
		"int16": -32767,
		"int32": -2147483647,
		"int64": -9223372036854775807
	}`
	require.JSONEq(expectedStructJSON, string(structJSON))

	objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(object, objectPacker.Packer)
	require.NoError(err)

	objectDigest := objectPacker.Bytes()

	require.Equal("fefffefffffffefffffffffffffffe818001800000018000000000000001", hex.EncodeToString(objectDigest))
}

type MockObjectStringAndBytes struct {
	Field1 string `serialize:"true" json:"field1"`
	Field2 []byte `serialize:"true" json:"field2"`
}

func (MockObjectStringAndBytes) GetTypeID() uint8 {
	return 4
}

func TestMarshalStringAndBytesSpec(t *testing.T) {
	require := require.New(t)

	testCases := []struct {
		name           string
		object         MockObjectStringAndBytes
		expectedJSON   string
		expectedDigest string
	}{
		{
			name: "Non-empty fields",
			object: MockObjectStringAndBytes{
				Field1: "Hello, World!",
				Field2: []byte{0x01, 0x02, 0x03, 0x04},
			},
			expectedJSON:   `{"field1": "Hello, World!","field2": "AQIDBA=="}`,
			expectedDigest: "000d48656c6c6f2c20576f726c64210000000401020304",
		},
		{
			name: "Empty fields",
			object: MockObjectStringAndBytes{
				Field1: "",
				Field2: []byte{},
			},
			expectedJSON:   `{"field1": "","field2": ""}`,
			expectedDigest: "000000000000",
		},
		{
			name: "String 'A' and empty bytes",
			object: MockObjectStringAndBytes{
				Field1: "A",
				Field2: []byte{},
			},
			expectedJSON:   `{"field1": "A","field2": ""}`,
			expectedDigest: "00014100000000",
		},
		{
			name: "Byte 0x00 and empty string",
			object: MockObjectStringAndBytes{
				Field1: "",
				Field2: []byte{0x00},
			},
			expectedJSON:   `{"field1": "","field2": "AA=="}`,
			expectedDigest: "00000000000100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			structJSON, err := json.Marshal(tc.object)
			require.NoError(err)
			require.JSONEq(tc.expectedJSON, string(structJSON))

			objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
			err = codec.LinearCodec.MarshalInto(tc.object, objectPacker.Packer)
			require.NoError(err)

			objectDigest := objectPacker.Bytes()
			require.Equal(tc.expectedDigest, hex.EncodeToString(objectDigest))
		})
	}
}

type MockObjectArrays struct {
	Strings []string `serialize:"true" json:"strings"`
	Bytes   [][]byte `serialize:"true" json:"bytes"`
	Uint8s  []uint8  `serialize:"true" json:"uint8s"`
	Uint16s []uint16 `serialize:"true" json:"uint16s"`
	Uint32s []uint32 `serialize:"true" json:"uint32s"`
	Uint64s []uint64 `serialize:"true" json:"uint64s"`
	Int8s   []int8   `serialize:"true" json:"int8s"`
	Int16s  []int16  `serialize:"true" json:"int16s"`
	Int32s  []int32  `serialize:"true" json:"int32s"`
	Int64s  []int64  `serialize:"true" json:"int64s"`
}

func (MockObjectArrays) GetTypeID() uint8 {
	return 5
}

func TestMarshalArraysSpec(t *testing.T) {
	require := require.New(t)

	object := MockObjectArrays{
		Strings: []string{"Hello", "World"},
		Bytes:   [][]byte{{0x01, 0x02}, {0x03, 0x04}},
		Uint8s:  []uint8{1, 2},
		Uint16s: []uint16{300, 400},
		Uint32s: []uint32{70000, 80000},
		Uint64s: []uint64{5000000000, 6000000000},
		Int8s:   []int8{-1, -2},
		Int16s:  []int16{-300, -400},
		Int32s:  []int32{-70000, -80000},
		Int64s:  []int64{-5000000000, -6000000000},
	}

	structJSON, err := json.Marshal(object)
	require.NoError(err)

	expectedStructJSON := `
	{
		"strings": ["Hello", "World"],
		"bytes": ["AQI=", "AwQ="],
		"uint8s": "AQI=",
		"uint16s": [300, 400],
		"uint32s": [70000, 80000],
		"uint64s": [5000000000, 6000000000],
		"int8s": [-1, -2],
		"int16s": [-300, -400],
		"int32s": [-70000, -80000],
		"int64s": [-5000000000, -6000000000]
	}`
	require.JSONEq(expectedStructJSON, string(structJSON))

	objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(object, objectPacker.Packer)
	require.NoError(err)

	objectDigest := objectPacker.Bytes()

	require.Equal("00000002000548656c6c6f0005576f726c640000000200000002010200000002030400000002010200000002012c019000000002000111700001388000000002000000012a05f2000000000165a0bc0000000002fffe00000002fed4fe7000000002fffeee90fffec78000000002fffffffed5fa0e00fffffffe9a5f4400", hex.EncodeToString(objectDigest))
}

func TestMarshalTransferSpec(t *testing.T) {
	require := require.New(t)

	object := MockActionTransfer{
		To:    codec.Address{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14},
		Value: 1000,
		Memo:  []byte("hi"),
	}

	structJSON, err := json.Marshal(object)
	require.NoError(err)

	addrString := codec.MustAddressBech32("morpheus", object.To)
	require.Equal("morpheus1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5qqqqqqqqqqqqqqqqqqqqqmqvs7e", addrString)

	expectedJSON := `{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"hi"}`
	require.Equal(expectedJSON, string(structJSON))

	objectPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(object, objectPacker.Packer)
	require.NoError(err)

	objectDigest := objectPacker.Bytes()
	expectedDigest := "0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e8000000026869"
	require.Equal(expectedDigest, hex.EncodeToString(objectDigest))
}

type MockActionWithTransfer struct {
	Transfer MockActionTransfer `serialize:"true" json:"transfer"`
}

func (MockActionWithTransfer) GetTypeID() uint8 {
	return 6
}

type MockActionWithTransferArray struct {
	Transfers []MockActionTransfer `serialize:"true" json:"transfers"`
}

func (MockActionWithTransferArray) GetTypeID() uint8 {
	return 7
}

func TestMarshalComplexStructs(t *testing.T) {
	require := require.New(t)

	transfer := MockActionTransfer{
		To:    codec.Address{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14},
		Value: 1000,
		Memo:  []byte("hi"),
	}

	// Struct with a single transfer
	actionWithTransfer := MockActionWithTransfer{
		Transfer: transfer,
	}
	structJSON, err := json.Marshal(actionWithTransfer)
	require.NoError(err)

	expectedJSON := `{"transfer":{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"hi"}}`
	require.JSONEq(expectedJSON, string(structJSON))

	actionPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(actionWithTransfer, actionPacker.Packer)
	require.NoError(err)

	actionDigest := actionPacker.Bytes()
	expectedDigest := "0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e8000000026869"
	require.Equal(expectedDigest, hex.EncodeToString(actionDigest))

	// Struct with an array of transfers
	actionWithTransferArray := MockActionWithTransferArray{
		Transfers: []MockActionTransfer{transfer, transfer},
	}
	structJSON, err = json.Marshal(actionWithTransferArray)
	require.NoError(err)

	expectedJSON = `{"transfers":[{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"hi"},{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"hi"}]}`
	require.JSONEq(expectedJSON, string(structJSON))

	actionPacker = codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(actionWithTransferArray, actionPacker.Packer)
	require.NoError(err)

	actionDigest = actionPacker.Bytes()
	expectedDigest = "000000020102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e80000000268690102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e8000000026869"
	require.Equal(expectedDigest, hex.EncodeToString(actionDigest))
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
