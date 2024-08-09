// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

// This is a combined spec for ABI and AutoMarshal
// The results of this test are used in TypeScript tests to ensure the TypeScript implementation is correct
// Tests are added on as-needed by Typescript base

var _ chain.Action = MockAction1{}

type MockAction1 struct {
	Field1 string
	Field2 int32
}

// ComputeUnits implements chain.Action.
func (s MockAction1) ComputeUnits(chain.Rules) uint64 {
	panic("ComputeUnits unimplemented")
}

// Execute implements chain.Action.
func (s MockAction1) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (outputs [][]byte, err error) {
	panic("Execute unimplemented")
}

// Size implements chain.Action.
func (s MockAction1) Size() int {
	//TODO: has to be automatic for automatic marshalling
	return codec.StringLen(s.Field1) + 4 //32 bit is 4 bytes
}

// StateKeys implements chain.Action.
func (s MockAction1) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	panic("StateKeys unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (s MockAction1) StateKeysMaxChunks() []uint16 {
	panic("StateKeysMaxChunks unimplemented")
}

// ValidRange implements chain.Action.
func (s MockAction1) ValidRange(chain.Rules) (start int64, end int64) {
	panic("ValidRange unimplemented")
}

func (s MockAction1) GetTypeID() uint8 {
	return 1
}

func TestMarshalSimpleSpec(t *testing.T) {
	require := require.New(t)

	abiString, err := codec.GetVmABIString([]codec.HavingTypeId{MockAction1{}})
	require.NoError(err)
	//this json will be input in TS
	require.JSONEq(`
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
		}
	]`, string(abiString))

	actionInstance := MockAction1{
		Field1: "Super value",
		Field2: -123777,
	}
	structJson, err := json.Marshal(actionInstance)
	require.NoError(err)

	//this json will also be an input in TS
	require.JSONEq(`
	{
	"Field1": "Super value",
	"Field2": -123777
	}`, string(structJson))

	//this digest hex would be an output of aformentioned 2 jsons
	tx := chain.Transaction{
		Base: &chain.Base{
			Timestamp: 123456789,
			ChainID:   [32]byte{},
			MaxFee:    0,
		},
		Actions: []chain.Action{actionInstance},
		Auth:    nil,
	}

	digest, err := tx.Digest()
	require.Equal(hex.EncodeToString(digest), "00000000075bcd15000000000000000000000000000000000000000000000000000000000000000000000000000000000101000b53757065722076616c7565fffe1c7f")
}
