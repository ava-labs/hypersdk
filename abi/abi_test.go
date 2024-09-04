// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

type Struct1 struct {
	Field1 string `serialize:"true"`
	Field2 int32  `serialize:"true"`
}

func (Struct1) GetTypeID() uint8 {
	return 1
}

func TestGetABIBasic(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{Struct1{}})
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
	]`, abiString)
}

func TestGetABIBasicPtr(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{&Struct1{}})
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
	]`, abiString)
}

type Transfer struct {
	To    codec.Address       `json:"to" serialize:"true"`
	Value uint64              `json:"value" serialize:"true"`
	Memo  codec.StringAsBytes `json:"memo,omitempty" serialize:"true"`
}

func (Transfer) GetTypeID() uint8 {
	return 2
}

func TestGetABITransfer(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{Transfer{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 2,
			"name": "Transfer",
			"types": {
				"Transfer": [
					{ "name": "to", "type": "Address" },
					{ "name": "value", "type": "uint64" },
					{ "name": "memo", "type": "StringAsBytes" }
				]
			}
		}
	]`, abiString)
}

type AllInts struct {
	Int8   int8   `serialize:"true"`
	Int16  int16  `serialize:"true"`
	Int32  int32  `serialize:"true"`
	Int64  int64  `serialize:"true"`
	Uint8  uint8  `serialize:"true"`
	Uint16 uint16 `serialize:"true"`
	Uint32 uint32 `serialize:"true"`
	Uint64 uint64 `serialize:"true"`
}

func (AllInts) GetTypeID() uint8 {
	return 3
}

func TestGetABIAllInts(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{AllInts{}})
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
	]`, abiString)
}

type InnerStruct struct {
	Field1 string `serialize:"true"`
	Field2 uint64 `serialize:"true"`
}

type OuterStructSingle struct {
	SingleItem InnerStruct `json:"single_item" serialize:"true"`
}

func (OuterStructSingle) GetTypeID() uint8 {
	return 4
}

func TestGetABIOuterStructSingle(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{OuterStructSingle{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 4,
			"name": "OuterStructSingle",
			"types": {
				"OuterStructSingle": [
					{
						"name": "single_item",
						"type": "InnerStruct"
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
	]`, abiString)
}

type OuterStructArray struct {
	Items []InnerStruct `json:"items" serialize:"true"`
}

func (OuterStructArray) GetTypeID() uint8 {
	return 5
}

func TestGetABIOuterStructArray(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{OuterStructArray{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 5,
			"name": "OuterStructArray",
			"types": {
				"OuterStructArray": [
					{
						"name": "items",
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
	]`, abiString)
}

type CompositionInner struct {
	InnerField1 uint64 `serialize:"true"`
}

func (CompositionInner) GetTypeID() uint8 {
	return 5
}

type CompositionOuter struct {
	CompositionInner `serialize:"true"`
	Field1           uint64 `serialize:"true"`
	Field2           string `serialize:"true"`
}

func (CompositionOuter) GetTypeID() uint8 {
	return 6
}

func TestGetABIComposition(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{CompositionOuter{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 6,
			"name": "CompositionOuter",
			"types": {
				"CompositionOuter": [
					{ "name": "InnerField1", "type": "uint64" },
					{ "name": "Field1", "type": "uint64" },
					{ "name": "Field2", "type": "string" }
				]
			}
		}
	]`, abiString)
}

type TestSerializeStruct struct {
	Field1 string `serialize:"true"`
	Field2 int    `serialize:"true"`
	Field3 bool
	Field4 float64 `serialize:"false"`
}

func (TestSerializeStruct) GetTypeID() uint8 {
	return 7
}

func TestSerializeFields(t *testing.T) {
	require := require.New(t)

	abiString, err := GetVMABIString([]codec.Typed{TestSerializeStruct{}})
	require.NoError(err)

	require.JSONEq(`[
		{
			"id": 7,
			"name": "TestSerializeStruct",
			"types": {
				"TestSerializeStruct": [
					{ "name": "Field1", "type": "string" },
					{ "name": "Field2", "type": "int" }
				]
			}
		}
	]`, abiString)
}
