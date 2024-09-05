package generated

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
	to    codec.Address       `serialize:"true"`
	value uint64              `serialize:"true"`
	memo  codec.StringAsBytes `serialize:"true"`
}

func (MockActionTransfer) GetTypeID() uint8 {
	return 2
}

type MockObjectAllNumbers struct {
	uint8  uint8  `serialize:"true"`
	uint16 uint16 `serialize:"true"`
	uint32 uint32 `serialize:"true"`
	uint64 uint64 `serialize:"true"`
	int8   int8   `serialize:"true"`
	int16  int16  `serialize:"true"`
	int32  int32  `serialize:"true"`
	int64  int64  `serialize:"true"`
}

func (MockObjectAllNumbers) GetTypeID() uint8 {
	return 3
}

type MockObjectStringAndBytes struct {
	field1 string  `serialize:"true"`
	field2 []uint8 `serialize:"true"`
}

func (MockObjectStringAndBytes) GetTypeID() uint8 {
	return 4
}

type MockObjectArrays struct {
	strings []string  `serialize:"true"`
	bytes   [][]uint8 `serialize:"true"`
	uint8s  []uint8   `serialize:"true"`
	uint16s []uint16  `serialize:"true"`
	uint32s []uint32  `serialize:"true"`
	uint64s []uint64  `serialize:"true"`
	int8s   []int8    `serialize:"true"`
	int16s  []int16   `serialize:"true"`
	int32s  []int32   `serialize:"true"`
	int64s  []int64   `serialize:"true"`
}

func (MockObjectArrays) GetTypeID() uint8 {
	return 5
}

type MockActionWithTransferArray struct {
	transfers []MockActionTransfer `serialize:"true"`
}

func (MockActionWithTransferArray) GetTypeID() uint8 {
	return 7
}

type MockActionTransfer struct {
	to    codec.Address       `serialize:"true"`
	value uint64              `serialize:"true"`
	memo  codec.StringAsBytes `serialize:"true"`
}

func (MockActionTransfer) GetTypeID() uint8 {
	return 7
}

type MockActionWithTransfer struct {
	transfer MockActionTransfer `serialize:"true"`
}

func (MockActionWithTransfer) GetTypeID() uint8 {
	return 6
}

type MockActionTransfer struct {
	to    codec.Address       `serialize:"true"`
	value uint64              `serialize:"true"`
	memo  codec.StringAsBytes `serialize:"true"`
}

func (MockActionTransfer) GetTypeID() uint8 {
	return 6
}
