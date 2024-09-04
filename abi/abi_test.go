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

	actualABI, err := GetVMABI([]codec.Typed{Struct1{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   1,
				Name: "Struct1",
				Types: map[string][]ABIField{
					"Struct1": {
						{Name: "Field1", Type: "string"},
						{Name: "Field2", Type: "int32"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
}

func TestGetABIBasicPtr(t *testing.T) {
	require := require.New(t)

	actualABI, err := GetVMABI([]codec.Typed{&Struct1{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   1,
				Name: "Struct1",
				Types: map[string][]ABIField{
					"Struct1": {
						{Name: "Field1", Type: "string"},
						{Name: "Field2", Type: "int32"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
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

	actualABI, err := GetVMABI([]codec.Typed{Transfer{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   2,
				Name: "Transfer",
				Types: map[string][]ABIField{
					"Transfer": {
						{Name: "to", Type: "Address"},
						{Name: "value", Type: "uint64"},
						{Name: "memo", Type: "StringAsBytes"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
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

	actualABI, err := GetVMABI([]codec.Typed{AllInts{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   3,
				Name: "AllInts",
				Types: map[string][]ABIField{
					"AllInts": {
						{Name: "Int8", Type: "int8"},
						{Name: "Int16", Type: "int16"},
						{Name: "Int32", Type: "int32"},
						{Name: "Int64", Type: "int64"},
						{Name: "Uint8", Type: "uint8"},
						{Name: "Uint16", Type: "uint16"},
						{Name: "Uint32", Type: "uint32"},
						{Name: "Uint64", Type: "uint64"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
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

	actualABI, err := GetVMABI([]codec.Typed{OuterStructSingle{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   4,
				Name: "OuterStructSingle",
				Types: map[string][]ABIField{
					"OuterStructSingle": {
						{Name: "single_item", Type: "InnerStruct"},
					},
					"InnerStruct": {
						{Name: "Field1", Type: "string"},
						{Name: "Field2", Type: "uint64"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
}

type OuterStructArray struct {
	Items []InnerStruct `json:"items" serialize:"true"`
}

func (OuterStructArray) GetTypeID() uint8 {
	return 5
}

func TestGetABIOuterStructArray(t *testing.T) {
	require := require.New(t)

	actualABI, err := GetVMABI([]codec.Typed{OuterStructArray{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   5,
				Name: "OuterStructArray",
				Types: map[string][]ABIField{
					"OuterStructArray": {
						{Name: "items", Type: "[]InnerStruct"},
					},
					"InnerStruct": {
						{Name: "Field1", Type: "string"},
						{Name: "Field2", Type: "uint64"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
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

	actualABI, err := GetVMABI([]codec.Typed{CompositionOuter{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   6,
				Name: "CompositionOuter",
				Types: map[string][]ABIField{
					"CompositionOuter": {
						{Name: "InnerField1", Type: "uint64"},
						{Name: "Field1", Type: "uint64"},
						{Name: "Field2", Type: "string"},
					},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
}

type TestSerializeSelectedFieldsStruct struct {
	Field1 string `serialize:"true"`
	Field2 int    `serialize:"true"`
	Field3 bool
	Field4 float64 `serialize:"false"`
}

func (TestSerializeSelectedFieldsStruct) GetTypeID() uint8 {
	return 7
}

func TestSerializeFields(t *testing.T) {
	require := require.New(t)

	actualABI, err := GetVMABI([]codec.Typed{TestSerializeSelectedFieldsStruct{}})
	require.NoError(err)

	expectedABI := VMABI{
		Actions: []SingleActionABI{
			{
				ID:   7,
				Name: "TestSerializeSelectedFieldsStruct",
				Types: map[string][]ABIField{
					"TestSerializeSelectedFieldsStruct": {{Name: "Field1", Type: "string"}, {Name: "Field2", Type: "int"}},
				},
			},
		},
	}

	require.Equal(expectedABI, actualABI)
}
