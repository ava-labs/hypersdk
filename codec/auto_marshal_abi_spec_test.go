// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

// Combined ABI and AutoMarshal spec
// Used to verify TypeScript implementation
// Tests added as needed by TypeScript
// Ensures consistency in marshaling, not testing Go struct marshaling itself

type AbstractMockAction struct {
}

func (s AbstractMockAction) ComputeUnits(chain.Rules) uint64 {
	panic("ComputeUnits unimplemented")
}
func (s AbstractMockAction) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (outputs [][]byte, err error) {
	panic("Execute unimplemented")
}
func (s AbstractMockAction) Size() int {
	// TODO: This has to be automatic for automatic marshalling
	return 0
}
func (s AbstractMockAction) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	panic("StateKeys unimplemented")
}
func (s AbstractMockAction) StateKeysMaxChunks() []uint16 {
	panic("StateKeysMaxChunks unimplemented")
}
func (s AbstractMockAction) ValidRange(chain.Rules) (start int64, end int64) {
	panic("ValidRange unimplemented")
}

type MockActionSingleNumber struct {
	AbstractMockAction
	Field1 uint16
}

func (s MockActionSingleNumber) GetTypeID() uint8 {
	return 1
}

type MockActionTransfer struct {
	AbstractMockAction
	To    codec.Address `json:"to"`
	Value uint64        `json:"value"`
	Memo  []byte        `json:"memo"`
}

func (s MockActionTransfer) GetTypeID() uint8 {
	return 2
}

func TestABISpec(t *testing.T) {
	require := require.New(t)

	VMActions := []codec.HavingTypeId{
		MockActionSingleNumber{},
		MockActionTransfer{},
		MockActionAllNumbers{},
		MockActionStringAndBytes{},
		MockActionArrays{},
		MockActionWithTransferArray{},
		MockActionWithTransfer{},
		MockActionWithTransferMap{},
	}
	abiString, err := codec.GetVmABIString(VMActions)
	require.NoError(err)
	// This JSON will be input in TypeScript
	expectedABI := `[
  {
    "id": 1,
    "name": "MockActionSingleNumber",
    "types": {
      "MockActionSingleNumber": [
        {
          "name": "Field1",
          "type": "uint16"
        }
      ]
    }
  },
  {
    "id": 2,
    "name": "MockActionTransfer",
    "types": {
      "MockActionTransfer": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ]
    }
  },
  {
    "id": 3,
    "name": "MockActionAllNumbers",
    "types": {
      "MockActionAllNumbers": [
        {
          "name": "uint8",
          "type": "uint8"
        },
        {
          "name": "uint16",
          "type": "uint16"
        },
        {
          "name": "uint32",
          "type": "uint32"
        },
        {
          "name": "uint64",
          "type": "uint64"
        },
        {
          "name": "int8",
          "type": "int8"
        },
        {
          "name": "int16",
          "type": "int16"
        },
        {
          "name": "int32",
          "type": "int32"
        },
        {
          "name": "int64",
          "type": "int64"
        }
      ]
    }
  },
  {
    "id": 4,
    "name": "MockActionStringAndBytes",
    "types": {
      "MockActionStringAndBytes": [
        {
          "name": "field1",
          "type": "string"
        },
        {
          "name": "field2",
          "type": "[]uint8"
        }
      ]
    }
  },
  {
    "id": 5,
    "name": "MockActionArrays",
    "types": {
      "MockActionArrays": [
        {
          "name": "strings",
          "type": "[]string"
        },
        {
          "name": "bytes",
          "type": "[][]uint8"
        },
        {
          "name": "uint8s",
          "type": "[]uint8"
        },
        {
          "name": "uint16s",
          "type": "[]uint16"
        },
        {
          "name": "uint32s",
          "type": "[]uint32"
        },
        {
          "name": "uint64s",
          "type": "[]uint64"
        },
        {
          "name": "int8s",
          "type": "[]int8"
        },
        {
          "name": "int16s",
          "type": "[]int16"
        },
        {
          "name": "int32s",
          "type": "[]int32"
        },
        {
          "name": "int64s",
          "type": "[]int64"
        }
      ]
    }
  },
  {
    "id": 7,
    "name": "MockActionWithTransferArray",
    "types": {
      "MockActionTransfer": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ],
      "MockActionWithTransferArray": [
        {
          "name": "transfers",
          "type": "[]MockActionTransfer"
        }
      ]
    }
  },
  {
    "id": 6,
    "name": "MockActionWithTransfer",
    "types": {
      "MockActionTransfer": [
        {
          "name": "to",
          "type": "Address"
        },
        {
          "name": "value",
          "type": "uint64"
        },
        {
          "name": "memo",
          "type": "[]uint8"
        }
      ],
      "MockActionWithTransfer": [
        {
          "name": "transfer",
          "type": "MockActionTransfer"
        }
      ]
    }
  },
  {
    "id": 8,
    "name": "MockActionWithTransferMap",
    "types": {
      "MockActionWithTransferMap": [
        {
          "name": "transfersMap",
          "type": ""
        }
      ]
    }
  }
]`
	require.Equal(expectedABI, string(abiString))

	abiHash := sha256.Sum256([]byte(abiString))
	require.Equal("1b11cc4be907c26b3aa8346c07866a86ec685f63198ccbe27937a4372c49a4bd", hex.EncodeToString(abiHash[:]))

}

func TestMarshalEmptySpec(t *testing.T) {
	require := require.New(t)

	var err error

	action1Instance := MockActionSingleNumber{
		Field1: 0,
	}
	structJSON, err := json.Marshal(action1Instance)
	require.NoError(err)

	// This JSON will also be an input in TypeScript
	expectedStructJSON := `
	{
		"Field1": 0
	}`
	require.JSONEq(expectedStructJSON, string(structJSON))

	// This is the output of the combination of above JSONs
	actionPacker := codec.NewWriter(action1Instance.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, action1Instance)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()

	require.Equal("0000", hex.EncodeToString(actionDigest))

}

func TestMarshalSingleNumberSpec(t *testing.T) {
	require := require.New(t)

	var err error

	action1Instance := MockActionSingleNumber{
		Field1: 12333,
	}
	structJSON, err := json.Marshal(action1Instance)
	require.NoError(err)

	// This JSON will also be an input in TypeScript
	expectedStructJSON := `
	{
		"Field1": 12333
	}`
	require.JSONEq(expectedStructJSON, string(structJSON))

	// This is the output of the combination of above JSONs
	actionPacker := codec.NewWriter(action1Instance.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, action1Instance)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()

	require.Equal("302d", hex.EncodeToString(actionDigest))

}

type MockActionAllNumbers struct {
	AbstractMockAction
	Uint8  uint8  `json:"uint8"`
	Uint16 uint16 `json:"uint16"`
	Uint32 uint32 `json:"uint32"`
	Uint64 uint64 `json:"uint64"`
	Int8   int8   `json:"int8"`
	Int16  int16  `json:"int16"`
	Int32  int32  `json:"int32"`
	Int64  int64  `json:"int64"`
}

func (s MockActionAllNumbers) GetTypeID() uint8 {
	return 3
}

func TestMarshalAllNumbersSpec(t *testing.T) {
	require := require.New(t)

	action := MockActionAllNumbers{
		Uint8:  254,
		Uint16: 65534,
		Uint32: 4294967294,
		Uint64: 18446744073709551614,
		Int8:   -127,
		Int16:  -32767,
		Int32:  -2147483647,
		Int64:  -9223372036854775807,
	}

	structJSON, err := json.Marshal(action)
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

	actionPacker := codec.NewWriter(action.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, action)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()

	require.Equal("fefffefffffffefffffffffffffffe818001800000018000000000000001", hex.EncodeToString(actionDigest))

}

type MockActionStringAndBytes struct {
	AbstractMockAction
	Field1 string `json:"field1"`
	Field2 []byte `json:"field2"`
}

func (s MockActionStringAndBytes) GetTypeID() uint8 {
	return 4
}
func TestMarshalStringAndBytesSpec(t *testing.T) {
	require := require.New(t)

	testCases := []struct {
		name           string
		action         MockActionStringAndBytes
		expectedJSON   string
		expectedDigest string
	}{
		{
			name: "Non-empty fields",
			action: MockActionStringAndBytes{
				Field1: "Hello, World!",
				Field2: []byte{0x01, 0x02, 0x03, 0x04},
			},
			expectedJSON:   `{"field1": "Hello, World!","field2": "AQIDBA=="}`,
			expectedDigest: "000d48656c6c6f2c20576f726c64210000000401020304",
		},
		{
			name: "Empty fields",
			action: MockActionStringAndBytes{
				Field1: "",
				Field2: []byte{},
			},
			expectedJSON:   `{"field1": "","field2": ""}`,
			expectedDigest: "000000000000",
		},
		{
			name: "String 'A' and empty bytes",
			action: MockActionStringAndBytes{
				Field1: "A",
				Field2: []byte{},
			},
			expectedJSON:   `{"field1": "A","field2": ""}`,
			expectedDigest: "00014100000000",
		},
		{
			name: "Byte 0x00 and empty string",
			action: MockActionStringAndBytes{
				Field1: "",
				Field2: []byte{0x00},
			},
			expectedJSON:   `{"field1": "","field2": "AA=="}`,
			expectedDigest: "00000000000100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			structJSON, err := json.Marshal(tc.action)
			require.NoError(err)
			require.JSONEq(tc.expectedJSON, string(structJSON))

			actionPacker := codec.NewWriter(tc.action.Size(), consts.NetworkSizeLimit)
			codec.AutoMarshalStruct(actionPacker, tc.action)
			require.NoError(actionPacker.Err())

			actionDigest := actionPacker.Bytes()
			require.Equal(tc.expectedDigest, hex.EncodeToString(actionDigest))
		})
	}
}

type MockActionArrays struct {
	AbstractMockAction
	Strings []string `json:"strings"`
	Bytes   [][]byte `json:"bytes"`
	Uint8s  []uint8  `json:"uint8s"`
	Uint16s []uint16 `json:"uint16s"`
	Uint32s []uint32 `json:"uint32s"`
	Uint64s []uint64 `json:"uint64s"`
	Int8s   []int8   `json:"int8s"`
	Int16s  []int16  `json:"int16s"`
	Int32s  []int32  `json:"int32s"`
	Int64s  []int64  `json:"int64s"`
}

func (s MockActionArrays) GetTypeID() uint8 {
	return 5
}

func TestMarshalArraysSpec(t *testing.T) {
	require := require.New(t)

	action := MockActionArrays{
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

	structJSON, err := json.Marshal(action)
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

	actionPacker := codec.NewWriter(action.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, action)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()

	require.Equal("0002000548656c6c6f0005576f726c6400020000000201020000000203040000000201020002012c0190000200011170000138800002000000012a05f2000000000165a0bc000002fffe0002fed4fe700002fffeee90fffec7800002fffffffed5fa0e00fffffffe9a5f4400", hex.EncodeToString(actionDigest))
}

func TestMarshalTransferSpec(t *testing.T) {
	require := require.New(t)

	action := MockActionTransfer{
		To:    codec.Address{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14},
		Value: 1000,
		Memo:  []byte{0x01, 0x02, 0x03},
	}

	structJSON, err := json.Marshal(action)
	require.NoError(err)

	expectedJSON := `{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"AQID"}`
	require.Equal(expectedJSON, string(structJSON))

	actionPacker := codec.NewWriter(action.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, action)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()
	expectedDigest := "0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203"
	require.Equal(expectedDigest, hex.EncodeToString(actionDigest))
}

type MockActionWithTransfer struct {
	AbstractMockAction
	Transfer MockActionTransfer `json:"transfer"`
}

func (s MockActionWithTransfer) GetTypeID() uint8 {
	return 6
}

type MockActionWithTransferArray struct {
	AbstractMockAction
	Transfers []MockActionTransfer `json:"transfers"`
}

func (s MockActionWithTransferArray) GetTypeID() uint8 {
	return 7
}

type MockActionWithTransferMap struct {
	AbstractMockAction
	TransfersMap map[string]MockActionTransfer `json:"transfersMap"`
}

func (s MockActionWithTransferMap) GetTypeID() uint8 {
	return 8
}

func TestMarshalComplexStructs(t *testing.T) {
	require := require.New(t)

	transfer := MockActionTransfer{
		To:    codec.Address{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14},
		Value: 1000,
		Memo:  []byte{0x01, 0x02, 0x03},
	}

	// Struct with a single transfer
	actionWithTransfer := MockActionWithTransfer{
		Transfer: transfer,
	}
	structJSON, err := json.Marshal(actionWithTransfer)
	require.NoError(err)

	expectedJSON := `{"transfer":{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"AQID"}}`
	require.JSONEq(expectedJSON, string(structJSON))

	actionPacker := codec.NewWriter(actionWithTransfer.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, actionWithTransfer)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()
	expectedDigest := "0102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203"
	require.Equal(expectedDigest, hex.EncodeToString(actionDigest))

	// Struct with an array of transfers
	actionWithTransferArray := MockActionWithTransferArray{
		Transfers: []MockActionTransfer{transfer, transfer},
	}
	structJSON, err = json.Marshal(actionWithTransferArray)
	require.NoError(err)

	expectedJSON = `{"transfers":[{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"AQID"},{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"AQID"}]}`
	require.JSONEq(expectedJSON, string(structJSON))

	actionPacker = codec.NewWriter(actionWithTransferArray.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, actionWithTransferArray)
	require.NoError(actionPacker.Err())

	actionDigest = actionPacker.Bytes()
	expectedDigest = "00020102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e8000000030102030102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203"
	require.Equal(expectedDigest, hex.EncodeToString(actionDigest))

	// Struct with a map of transfers
	actionWithTransferMap := MockActionWithTransferMap{
		TransfersMap: map[string]MockActionTransfer{
			"first":  transfer,
			"second": transfer,
		},
	}
	structJSON, err = json.Marshal(actionWithTransferMap)
	require.NoError(err)

	expectedJSON = `{"transfersMap":{"first":{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"AQID"},"second":{"to":"AQIDBAUGBwgJCgsMDQ4PEBESExQAAAAAAAAAAAAAAAAA","value":1000,"memo":"AQID"}}}`
	require.JSONEq(expectedJSON, string(structJSON))

	actionPacker = codec.NewWriter(actionWithTransferMap.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, actionWithTransferMap)
	require.NoError(actionPacker.Err())

	actionDigest = actionPacker.Bytes()
	expectedDigest = "0002000566697273740102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e80000000301020300067365636f6e640102030405060708090a0b0c0d0e0f10111213140000000000000000000000000000000000000003e800000003010203"
	require.Equal(expectedDigest, hex.EncodeToString(actionDigest))
}
