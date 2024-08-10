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
  }
]`
	require.Equal(expectedABI, string(abiString))

	abiHash := sha256.Sum256([]byte(abiString))
	require.Equal("9ca568711bf22f818756c7c552e1aa012ffd728d55ea33241f9636173a532f07", hex.EncodeToString(abiHash[:]))

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
