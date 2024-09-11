// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"github.com/ava-labs/hypersdk/codec"
)

type MockObjectSingleNumber struct {
	Field1 uint16 `serialize:"true"`
}

func (MockObjectSingleNumber) GetTypeID() uint8 {
	return 1
}

type MockActionTransfer struct {
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Memo  codec.Bytes   `serialize:"true" json:"memo"`
}

func (MockActionTransfer) GetTypeID() uint8 {
	return 2
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

type MockObjectStringAndBytes struct {
	Field1 string  `serialize:"true" json:"field1"`
	Field2 []uint8 `serialize:"true" json:"field2"`
}

func (MockObjectStringAndBytes) GetTypeID() uint8 {
	return 4
}

type MockObjectArrays struct {
	Strings []string  `serialize:"true" json:"strings"`
	Bytes   [][]uint8 `serialize:"true" json:"bytes"`
	Uint8s  []uint8   `serialize:"true" json:"uint8s"`
	Uint16s []uint16  `serialize:"true" json:"uint16s"`
	Uint32s []uint32  `serialize:"true" json:"uint32s"`
	Uint64s []uint64  `serialize:"true" json:"uint64s"`
	Int8s   []int8    `serialize:"true" json:"int8s"`
	Int16s  []int16   `serialize:"true" json:"int16s"`
	Int32s  []int32   `serialize:"true" json:"int32s"`
	Int64s  []int64   `serialize:"true" json:"int64s"`
}

func (MockObjectArrays) GetTypeID() uint8 {
	return 5
}

type MockActionWithTransferArray struct {
	Transfers []MockActionTransfer `serialize:"true" json:"transfers"`
}

func (MockActionWithTransferArray) GetTypeID() uint8 {
	return 7
}

type MockActionWithTransfer struct {
	Transfer MockActionTransfer `serialize:"true" json:"transfer"`
}

func (MockActionWithTransfer) GetTypeID() uint8 {
	return 6
}

type Outer struct {
	Inner    Inner   `serialize:"true" json:"inner"`
	InnerArr []Inner `serialize:"true" json:"innerArr"`
}

func (Outer) GetTypeID() uint8 {
	return 8
}

type Inner struct {
	Field1 uint8 `serialize:"true" json:"field1"`
}

type ActionWithOutput struct {
	Field1 uint8 `serialize:"true" json:"field1"`
}

func (ActionWithOutput) GetTypeID() uint8 {
	return 9
}

// write action output type into the outputTypeField
type ActionOutput struct {
	Field1 uint16 `serialize:"true" json:"field1"`
}
