// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import "github.com/ava-labs/hypersdk/codec"

type MockObjectSingleNumber struct {
	Field1 uint16 `serialize:"true"`
}

type MockActionTransfer struct {
	To    codec.Address `serialize:"true" json:"to"`
	Value uint64        `serialize:"true" json:"value"`
	Memo  []uint8       `serialize:"true" json:"memo"`
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

type MockObjectStringAndBytes struct {
	Field1 string  `serialize:"true" json:"field1"`
	Field2 []uint8 `serialize:"true" json:"field2"`
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

type MockActionWithTransfer struct {
	Transfer MockActionTransfer `serialize:"true" json:"transfer"`
}

type MockActionWithTransferArray struct {
	Transfers []MockActionTransfer `serialize:"true" json:"transfers"`
}

type Outer struct {
	Inner    Inner   `serialize:"true" json:"inner"`
	InnerArr []Inner `serialize:"true" json:"innerArr"`
}

type Inner struct {
	Field1 uint8 `serialize:"true" json:"field1"`
}

type ActionWithOutput struct {
	Field1 uint8 `serialize:"true" json:"field1"`
}

type FixedBytes struct {
	TwoBytes       [2]uint8  `serialize:"true" json:"twoBytes"`
	ThirtyTwoBytes [32]uint8 `serialize:"true" json:"thirtyTwoBytes"`
}

type Bools struct {
	Bool1     bool   `serialize:"true" json:"bool1"`
	Bool2     bool   `serialize:"true" json:"bool2"`
	BoolArray []bool `serialize:"true" json:"boolArray"`
}

type ActionOutput struct {
	Field1 uint16 `serialize:"true" json:"field1"`
}

func (MockObjectSingleNumber) GetTypeID() uint8 {
	return 0
}

func (MockActionTransfer) GetTypeID() uint8 {
	return 1
}

func (MockObjectAllNumbers) GetTypeID() uint8 {
	return 2
}

func (MockObjectStringAndBytes) GetTypeID() uint8 {
	return 3
}

func (MockObjectArrays) GetTypeID() uint8 {
	return 4
}

func (MockActionWithTransfer) GetTypeID() uint8 {
	return 5
}

func (MockActionWithTransferArray) GetTypeID() uint8 {
	return 6
}

func (Outer) GetTypeID() uint8 {
	return 7
}

func (ActionWithOutput) GetTypeID() uint8 {
	return 8
}

func (FixedBytes) GetTypeID() uint8 {
	return 9
}

func (Bools) GetTypeID() uint8 {
	return 10
}

func (ActionOutput) GetTypeID() uint8 {
	return 0
}
