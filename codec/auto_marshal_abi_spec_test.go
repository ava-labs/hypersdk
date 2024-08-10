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

func TestMarshalSimpleSpec(t *testing.T) {
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

func TestABISpec(t *testing.T) {
	require := require.New(t)

	VMActions := []codec.HavingTypeId{
		MockActionSingleNumber{},
		MockActionTransfer{},
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
  }
]`
	require.Equal(expectedABI, string(abiString))

	abiHash := sha256.Sum256([]byte(abiString))
	require.Equal("a92c32c95198ea6539871193f2f45187d89378de0c6bd0095f5ff3b79557f34e", hex.EncodeToString(abiHash[:]))

}
