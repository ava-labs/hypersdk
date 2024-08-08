package codec_test

import (
	"testing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

type Struct1 struct {
	Field1 string
	Field2 int32
}

func (s Struct1) GetTypeID() uint8 {
	return 1
}
func TestGetABIBasic(t *testing.T) {
	require := require.New(t)

	abiString, err := codec.GetVmABIString([]codec.HavingTypeId{Struct1{}})
	require.NoError(err)
	require.JSONEq(`[
		{
			"id": 1,
			"name": "Struct1",
			"types": {
				"Struct1": [
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
}

func TestGetABIBasicPtr(t *testing.T) {
	require := require.New(t)

	abiString, err := codec.GetVmABIString([]codec.HavingTypeId{&Struct1{}})
	require.NoError(err)
	require.JSONEq(`[
		{
			"id": 1,
			"name": "Struct1",
			"types": {
				"Struct1": [
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
}

type Transfer struct {
	To    codec.Address `json:"to"`
	Value uint64        `json:"value"`
	Memo  []byte        `json:"memo,omitempty"`
}

func (t Transfer) GetTypeID() uint8 {
	return 2
}

func TestGetABITransfer(t *testing.T) {
	require := require.New(t)

	abiString, err := codec.GetVmABIString([]codec.HavingTypeId{Transfer{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 2,
			"name": "Transfer",
			"types": {
				"Transfer": [
					{ "name": "to", "type": "Address" },
					{ "name": "value", "type": "uint64" },
					{ "name": "memo", "type": "[]uint8" }
				]
			}
		}
	]`, string(abiString))
}

type AllInts struct {
	Int8   int8
	Int16  int16
	Int32  int32
	Int64  int64
	Uint8  uint8
	Uint16 uint16
	Uint32 uint32
	Uint64 uint64
}

func (a AllInts) GetTypeID() uint8 {
	return 3
}

func TestGetABIAllInts(t *testing.T) {
	require := require.New(t)

	abiString, err := codec.GetVmABIString([]codec.HavingTypeId{AllInts{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 3,
			"name": "AllInts",
			"types": {
				"AllInts": [
					{ "name": "Int8", "type": "int8" },
					{ "name": "Int16", "type": "int16" },
					{ "name": "Int32", "type": "int32" },
					{ "name": "Int64", "type": "int64" },
					{ "name": "Uint8", "type": "uint8" },
					{ "name": "Uint16", "type": "uint16" },
					{ "name": "Uint32", "type": "uint32" },
					{ "name": "Uint64", "type": "uint64" }
				]
			}
		}
	]`, string(abiString))
}

type InnerStruct struct {
	Field1 string
	Field2 uint64
}

type OuterStruct struct {
	SingleItem InnerStruct   `json:"single_item"`
	ArrayItems []InnerStruct `json:"array_items"`
}

func (o OuterStruct) GetTypeID() uint8 {
	return 4
}

func TestGetABIOuterStruct(t *testing.T) {
	require := require.New(t)

	abiString, err := codec.GetVmABIString([]codec.HavingTypeId{OuterStruct{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 4,
			"name": "OuterStruct",
			"types": {
				"OuterStruct": [
					{
						"name": "single_item",
						"type": "InnerStruct"
					},
					{
						"name": "array_items",
						"type": "[]InnerStruct"
					}
				],
				"InnerStruct": [
					{
						"name": "Field1",
						"type": "string"
					},
					{
						"name": "Field2",
						"type": "uint64"
					}
				]
			}
		}
	]`, string(abiString))
}
