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

type MockAction1 struct {
	AbstractMockAction
	Field1 string
	Field2 int32
}

func (s MockAction1) GetTypeID() uint8 {
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

func TestMarshalSimpleSpec(t *testing.T) {
	require := require.New(t)

	VMActions := []codec.HavingTypeId{
		MockAction1{},
		MockActionTransfer{},
	}
	abiString, err := codec.GetVmABIString(VMActions)
	require.NoError(err)
	// This JSON will be input in TypeScript
	expectedABI := `
	[
		{
			"id": 1,
			"name": "MockAction1",
			"types": {
				"MockAction1": [
					{
						"name": "Field1",
						"type": "string"
					},
					{
						"name": "Field2",
						"type": "int32"
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
	require.JSONEq(expectedABI, string(abiString))

	action1Instance := MockAction1{
		Field1: "Super value",
		Field2: -123777,
	}
	structJSON, err := json.Marshal(action1Instance)
	require.NoError(err)

	// This JSON will also be an input in TypeScript
	expectedStructJSON := `
	{
		"Field1": "Super value",
		"Field2": -123777
	}`
	require.JSONEq(expectedStructJSON, string(structJSON))

	// This is the output of the combination of above JSONs
	actionPacker := codec.NewWriter(action1Instance.Size(), consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(actionPacker, action1Instance)
	require.NoError(actionPacker.Err())

	actionDigest := actionPacker.Bytes()

	require.Equal("000b53757065722076616c7565fffe1c7f", hex.EncodeToString(actionDigest))

	abiHash := sha256.Sum256([]byte(abiString))
	require.Equal("404e365ee910729071642fa843076186ad6001a6314cde7c0ae5f1355f90ab8e", hex.EncodeToString(abiHash[:]))
}
