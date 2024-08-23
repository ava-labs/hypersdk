// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/linearcodeccopy"
	"github.com/ava-labs/hypersdk/consts"

	"github.com/ava-labs/avalanchego/codec/hierarchycodec"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	reflect "reflect"
)

func TestMarshalTransfer(t *testing.T) {
	type testStructure struct {
		// To is the recipient of the [Value].
		To codec.Address `json:"to"`

		// Amount are transferred to [To].
		Value uint64 `json:"value"`

		// Optional message to accompany transaction.
		Memo []byte `json:"memo"`
	}

	transfer := testStructure{
		To:    codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Value: 12876198273671286,
		Memo:  []byte("Hello World"),
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	// this is a copy of actions.Transfer.Marshal() logic
	packer.PackAddress(transfer.To)
	packer.PackUint64(transfer.Value)
	packer.PackBytes(transfer.Memo)
	expectedBytes := packer.Bytes()

	packer = codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, transfer)
	require.NoError(t, packer.Err())

	require.Equal(t, expectedBytes, packer.Bytes())

	// unmarshal
	var restoredStruct testStructure
	err := codec.AutoUnmarshalStruct(codec.NewReader(expectedBytes, len(expectedBytes)), &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, transfer, restoredStruct)

}

func TestMarshalTransferViaStandardReflectCodec(t *testing.T) {
	require := require.New(t)
	type testStructure struct {
		// To is the recipient of the [Value].
		To codec.Address `json:"to" serialize:"true"`

		// Amount are transferred to [To].
		Value uint64 `json:"value" serialize:"true"`

		// Optional message to accompany transaction.
		Memo []byte `json:"memo" serialize:"true"`
	}

	transfer := testStructure{
		To:    codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Value: 12876198273671286,
		Memo:  []byte("Hello World"),
	}

	manualPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	// this is a copy of actions.Transfer.Marshal() logic
	manualPacker.PackAddress(transfer.To)
	manualPacker.PackUint64(transfer.Value)
	manualPacker.PackBytes(transfer.Memo)
	expectedBytes := manualPacker.Bytes()

	codecInstance := hierarchycodec.NewDefault()

	p := wrappers.Packer{
		MaxSize: consts.NetworkSizeLimit,
		Bytes:   make([]byte, 0, 128),
	}
	err := codecInstance.MarshalInto(transfer, &p)
	require.NoError(err)

	myStructBytes := p.Bytes

	var restoredStruct testStructure
	err = codecInstance.Unmarshal(myStructBytes, &restoredStruct)
	require.NoError(err)

	require.Equal(transfer, restoredStruct)
	require.Equal(expectedBytes, myStructBytes)
}

func TestMarshalTransferViaReflectCodecCopy(t *testing.T) {
	require := require.New(t)
	type testStructure struct {
		// To is the recipient of the [Value].
		To codec.Address `json:"to" serialize:"true"`

		// Amount are transferred to [To].
		Value uint64 `json:"value" serialize:"true"`

		// Optional message to accompany transaction.
		Memo []byte `json:"memo" serialize:"true"`
	}

	transfer := testStructure{
		To:    codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Value: 12876198273671286,
		Memo:  []byte("Hello World"),
	}

	manualPacker := codec.NewWriter(0, consts.NetworkSizeLimit)
	// this is a copy of actions.Transfer.Marshal() logic
	manualPacker.PackAddress(transfer.To)
	manualPacker.PackUint64(transfer.Value)
	manualPacker.PackBytes(transfer.Memo)
	expectedBytes := manualPacker.Bytes()

	codecInstance := linearcodeccopy.NewDefault()

	p := wrappers.Packer{
		MaxSize: consts.NetworkSizeLimit,
		Bytes:   make([]byte, 0, 128),
	}
	err := codecInstance.MarshalInto(transfer, &p)
	require.NoError(err)

	myStructBytes := p.Bytes

	var restoredStruct testStructure
	p.Offset = 0 // reset to read from the start
	err = codecInstance.UnmarshalPacker(&p, &restoredStruct)
	require.NoError(err)

	require.Equal(transfer, restoredStruct)
	require.Equal(expectedBytes, myStructBytes)
}

func TestMarshalNumber(t *testing.T) {
	type NumberStructure struct {
		NegIntOne   int
		NegInt8One  int8
		NegInt16One int16
		NegInt32One int32
		NegInt64One int64
		NegIntMin   int
		NegInt8Min  int8
		NegInt16Min int16
		NegInt32Min int32
		NegInt64Min int64
		UintZero    uint
		Uint8Zero   uint8
		Uint16Zero  uint16
		Uint32Zero  uint32
		Uint64Zero  uint64
		UintMax     uint
		Uint8Max    uint8
		Uint16Max   uint16
		Uint32Max   uint32
		Uint64Max   uint64
		IntMax      int
		Int8Max     int8
		Int16Max    int16
		Int32Max    int32
		Int64Max    int64
	}

	test := NumberStructure{
		NegIntOne:   -1,
		NegInt8One:  -1,
		NegInt16One: -1,
		NegInt32One: -1,
		NegInt64One: -1,
		NegIntMin:   math.MinInt,
		NegInt8Min:  math.MinInt8,
		NegInt16Min: math.MinInt16,
		NegInt32Min: math.MinInt32,
		NegInt64Min: math.MinInt64,
		UintZero:    0,
		Uint8Zero:   0,
		Uint16Zero:  0,
		Uint32Zero:  0,
		Uint64Zero:  0,
		UintMax:     ^uint(0),
		Uint8Max:    math.MaxUint8,
		Uint16Max:   math.MaxUint16,
		Uint32Max:   math.MaxUint32,
		Uint64Max:   math.MaxUint64,
		IntMax:      math.MaxInt,
		Int8Max:     math.MaxInt8,
		Int16Max:    math.MaxInt16,
		Int32Max:    math.MaxInt32,
		Int64Max:    math.MaxInt64,
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.NoError(t, packer.Err())

	var restoredStruct NumberStructure
	err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), 0), &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func TestMarshalFlatTypes(t *testing.T) {
	type FlatStructure struct {
		IntField       int           `json:"intField"`
		Int8Field      int8          `json:"int8Field"`
		Int16Field     int16         `json:"int16Field"`
		Int32Field     int32         `json:"int32Field"`
		Int64Field     int64         `json:"int64Field"`
		UintField      uint          `json:"uintField"`
		Uint8Field     uint8         `json:"uint8Field"`
		Uint16Field    uint16        `json:"uint16Field"`
		Uint32Field    uint32        `json:"uint32Field"`
		Uint64Field    uint64        `json:"uint64Field"`
		StringField    string        `json:"stringField"`
		AddressField   codec.Address `json:"addressField"`
		ByteArrayField []byte        `json:"byteArrayField"`
	}

	test := FlatStructure{
		IntField:       42,
		Int8Field:      8,
		Int16Field:     16,
		Int32Field:     32,
		Int64Field:     64,
		UintField:      42,
		Uint8Field:     8,
		Uint16Field:    16,
		Uint32Field:    32,
		Uint64Field:    64,
		StringField:    "Test String",
		AddressField:   codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
		ByteArrayField: []byte{10, 20, 30},
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.NoError(t, packer.Err())

	var restoredStruct FlatStructure
	err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func TestMarshalEmptyFlatTypes(t *testing.T) {
	type FlatStructure struct {
		IntField       int    `json:"intField"`
		Int8Field      int8   `json:"int8Field"`
		Int16Field     int16  `json:"int16Field"`
		Int32Field     int32  `json:"int32Field"`
		Int64Field     int64  `json:"int64Field"`
		UintField      uint   `json:"uintField"`
		Uint8Field     uint8  `json:"uint8Field"`
		Uint16Field    uint16 `json:"uint16Field"`
		Uint32Field    uint32 `json:"uint32Field"`
		Uint64Field    uint64 `json:"uint64Field"`
		StringField    string `json:"stringField"`
		ByteArrayField []byte `json:"byteArrayField"`
	}

	test := FlatStructure{
		ByteArrayField: []byte{}, // codec would unmarshal nil to []byte{} anyway
	}
	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	bytes := packer.Bytes()

	var restoredStruct FlatStructure
	err := codec.AutoUnmarshalStruct(codec.NewReader(bytes, consts.NetworkSizeLimit), &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func TestMarshalStructWithArrayOfStructs(t *testing.T) {
	type SimpleStruct struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Value uint64 `json:"value"`
	}

	type ComplexStruct struct {
		Title            string                  `json:"title"`
		Description      string                  `json:"description"`
		Items            []SimpleStruct          `json:"items"`
		MapField         map[string]SimpleStruct `json:"mapField"`
		EmptyTitle       string                  `json:"emptyTitle"`
		EmptyDescription string                  `json:"emptyDescription"`
		EmptyItems       []SimpleStruct          `json:"emptyItems"`
		EmptyMapField    map[string]SimpleStruct `json:"emptyMapField"`
	}

	test := ComplexStruct{
		Title:       "Test Complex Struct",
		Description: "This is a test of a struct containing an array of other structs",
		Items: []SimpleStruct{
			{ID: 1, Name: "Item 1", Value: 100},
			{ID: 2, Name: "Item 2", Value: 200},
			{ID: 3, Name: "Item 3", Value: 300},
		},
		MapField: map[string]SimpleStruct{
			"key1": {ID: 4, Name: "Item 4", Value: 400},
			"key2": {ID: 5, Name: "Item 5", Value: 500},
		},
		EmptyTitle:       "",
		EmptyDescription: "",
		EmptyItems:       []SimpleStruct{},
		EmptyMapField:    map[string]SimpleStruct{},
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.NoError(t, packer.Err())

	var restoredStruct ComplexStruct
	err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)

	// Additional checks for nested structures
	require.Equal(t, len(test.Items), len(restoredStruct.Items))
	for i, item := range test.Items {
		require.Equal(t, item, restoredStruct.Items[i])
	}

	require.Equal(t, len(test.MapField), len(restoredStruct.MapField))
	for key, value := range test.MapField {
		require.Equal(t, value, restoredStruct.MapField[key])
	}

	require.Empty(t, restoredStruct.EmptyTitle)
	require.Empty(t, restoredStruct.EmptyDescription)
	require.Empty(t, restoredStruct.EmptyItems)
	require.Empty(t, restoredStruct.EmptyMapField)
}

func TestMarshalSizes(t *testing.T) {
	tests := []struct {
		name         string
		value        interface{}
		expectedSize int
	}{
		{
			name: "Basic types",
			value: struct {
				Uint8Field  uint8
				Uint16Field uint16
				Uint32Field uint32
				Uint64Field uint64
				Int8Field   int8
				Int16Field  int16
				Int32Field  int32
				Int64Field  int64
				BoolField   bool
			}{
				Uint8Field:  255,
				Uint16Field: 65535,
				Uint32Field: 4294967295,
				Uint64Field: 18446744073709551615,
				Int8Field:   -128,
				Int16Field:  -32768,
				Int32Field:  -2147483648,
				Int64Field:  -9223372036854775808,
				BoolField:   true,
			},
			expectedSize: 1 + 2 + 4 + 8 + 1 + 2 + 4 + 8 + 1,
		},
		{
			name: "String fields",
			value: struct {
				EmptyString  string
				ShortString  string
				LongerString string
			}{
				EmptyString:  "",
				ShortString:  "123",
				LongerString: "Hello, World!",
			},
			expectedSize: (2 + 0) + (2 + 3) + (2 + 13),
		},
		{
			name: "Byte slice fields",
			value: struct {
				EmptySlice  []byte
				ShortSlice  []byte
				LongerSlice []byte
			}{
				EmptySlice:  []byte{},
				ShortSlice:  []byte{1, 2, 3},
				LongerSlice: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			},
			expectedSize: (0 + 4) + (3 + 4) + (10 + 4),
		},
		{
			name: "Address field",
			value: struct {
				AddressField codec.Address
			}{
				AddressField: codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
			expectedSize: codec.AddressLen,
		},
		{
			name: "Mixed fields",
			value: struct {
				IntField     int32
				StringField  string
				BytesField   []byte
				BoolField    bool
				AddressField codec.Address
			}{
				IntField:     42,
				StringField:  "Test",
				BytesField:   []byte{1, 2, 3},
				BoolField:    true,
				AddressField: codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
			},
			expectedSize: 4 + (4 + 2) + (3 + 4) + 1 + codec.AddressLen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packer := codec.NewWriter(0, consts.NetworkSizeLimit)
			codec.AutoMarshalStruct(packer, tt.value)
			require.NoError(t, packer.Err())
			bytes := packer.Bytes()

			// Check size
			require.Len(t, bytes, tt.expectedSize, "Encoded size mismatch")

			// Decode and compare
			decoded := reflect.New(reflect.TypeOf(tt.value)).Interface()
			err := codec.AutoUnmarshalStruct(codec.NewReader(bytes, consts.NetworkSizeLimit), decoded)
			require.NoError(t, err)

			require.Equal(t, tt.value, reflect.ValueOf(decoded).Elem().Interface(), "Decoded value mismatch")
		})
	}
}

func TestAdditionalCornerCases(t *testing.T) {
	t.Run("NestedStructs", func(t *testing.T) {
		type Inner struct {
			Field int
		}
		type Middle struct {
			Inner Inner
		}
		type Outer struct {
			Middle Middle
		}
		test := Outer{Middle: Middle{Inner: Inner{Field: 42}}}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored Outer
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)

		require.Equal(t, test, restored)
	})

	t.Run("EmptyStruct", func(t *testing.T) {
		type Empty struct{}
		test := Empty{}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored Empty
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)

		require.Equal(t, test, restored)
	})

	t.Run("UnexportedFields", func(t *testing.T) {
		type Mixed struct {
			Exported   int
			unexported string
		}
		test := Mixed{Exported: 42, unexported: "should be ignored"}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored Mixed
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)

		require.Equal(t, test.Exported, restored.Exported)
		require.Empty(t, restored.unexported)
	})

	t.Run("StructWithPointers", func(t *testing.T) {
		type PointerStruct struct {
			IntPtr    *int
			StringPtr *string
		}
		intVal := 42
		strVal := "test"
		test := PointerStruct{IntPtr: &intVal, StringPtr: &strVal}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.ErrorIs(t, packer.Err(), codec.ErrUnsupportedFieldType)
	})

	t.Run("PointerToStruct", func(t *testing.T) {
		type MyStruct struct {
			IntVal    int
			StringVal string
		}
		intVal := 42
		strVal := "test"
		test := MyStruct{IntVal: intVal, StringVal: strVal}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored MyStruct
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)
		require.Equal(t, test, restored)
	})
	t.Run("EmbeddedStruct", func(t *testing.T) {
		type Embedded struct {
			Field int
		}
		type Outer struct {
			Embedded
			OtherField string
		}
		test := Outer{Embedded: Embedded{Field: 42}, OtherField: "test"}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored Outer
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)

		require.Equal(t, test, restored)
	})

	t.Run("UnexportedEmbeddedStruct", func(t *testing.T) {
		type embedded struct {
			field int
		}
		type Outer struct {
			embedded
			OtherField string
		}
		test := Outer{embedded: embedded{field: 42}, OtherField: "test"}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored Outer
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)

		require.Equal(t, test.OtherField, restored.OtherField)
		require.Zero(t, restored.embedded.field) // Unexported fields should be ignored
	})

	t.Run("StructWithInterface", func(t *testing.T) {
		type Struct struct {
			Field interface{}
		}
		test := Struct{Field: "test"}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.ErrorIs(t, packer.Err(), codec.ErrUnsupportedFieldType)
	})

	t.Run("CustomType", func(t *testing.T) {
		type CustomInt int
		type Struct struct {
			Field CustomInt
		}
		test := Struct{Field: CustomInt(42)}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, &test)
		require.NoError(t, packer.Err())

		var restored Struct
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		require.NoError(t, err)

		require.Equal(t, test, restored)
	})
}

func TestMarshalLengths(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int
	}{
		{"Bool", struct{ V bool }{true}, 1},
		{"Int8", struct{ V int8 }{127}, 1},
		{"Int16", struct{ V int16 }{32767}, 2},
		{"Int32", struct{ V int32 }{2147483647}, 4},
		{"Int64", struct{ V int64 }{9223372036854775807}, 8},
		{"Uint8", struct{ V uint8 }{255}, 1},
		{"Uint16", struct{ V uint16 }{65535}, 2},
		{"Uint32", struct{ V uint32 }{4294967295}, 4},
		{"Uint64", struct{ V uint64 }{18446744073709551615}, 8},
		{"Bytes", struct{ V []byte }{[]byte{1, 2, 3}}, 3 + 4},
		{"Uint16ArrayEmpty", struct{ V []uint16 }{[]uint16{}}, 2},
		{"Uint16Array", struct{ V []uint16 }{[]uint16{1, 2, 3}}, 3*2 + 2},
		{"String", struct{ V string }{"hello"}, 5 + 2},
		{"StringArray", struct{ V []string }{[]string{"hello", "world"}}, (5 + 2) + (5 + 2) + 2},
		{"BytesArray", struct{ V [][]byte }{[][]byte{{1, 2, 3}, {3, 4, 5}}}, (3 + 4) + (3 + 4) + 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packer := codec.NewWriter(0, consts.NetworkSizeLimit)
			codec.AutoMarshalStruct(packer, tt.value)
			require.NoError(t, packer.Err())
			bytes := packer.Bytes()

			require.Len(t, bytes, tt.expected, "Expected length %d, got %d")

			restored := reflect.New(reflect.TypeOf(tt.value)).Interface()
			err := codec.AutoUnmarshalStruct(codec.NewReader(bytes, consts.NetworkSizeLimit), restored)
			require.NoError(t, err)

			require.True(t, reflect.DeepEqual(tt.value, reflect.ValueOf(restored).Elem().Interface()),
				"Value mismatch. Expected %v, got %v", tt.value, reflect.ValueOf(restored).Elem().Interface())
		})
	}
}

func TestMarshalLongBytes(t *testing.T) {
	type LongBytesStruct struct {
		LongBytes []byte
	}

	longBytes := make([]byte, 70000)
	for i := range longBytes {
		longBytes[i] = byte(i % 256)
	}

	test := LongBytesStruct{
		LongBytes: longBytes,
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.NoError(t, packer.Err())
	bytes := packer.Bytes()

	// 70000 bytes for the data + 4 bytes for the length prefix
	expectedLength := 70000 + 4
	require.Len(t, bytes, expectedLength, "Expected marshaled length %d, got %d", expectedLength, len(bytes))

	var restored LongBytesStruct
	err := codec.AutoUnmarshalStruct(codec.NewReader(bytes, consts.NetworkSizeLimit), &restored)
	require.NoError(t, err)

	require.Equal(t, test, restored, "Restored value does not match original")
}

func TestMarshalLongArray(t *testing.T) {
	type LongArrayStruct struct {
		LongArray []uint16
	}

	// Create an array that's too long (more than 65535 elements)
	longArray := make([]uint16, math.MaxUint16+1)
	for i := range longArray {
		longArray[i] = uint16(i % math.MaxUint16)
	}

	test := LongArrayStruct{
		LongArray: longArray,
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.ErrorIs(t, packer.Err(), codec.ErrTooManyItems)
}

func TestMarshalLongMap(t *testing.T) {
	type LongMapStruct struct {
		LongMap map[uint64]uint64
	}

	longMap := map[uint64]uint64{}
	for i := uint32(0); i <= math.MaxUint16+1; i++ {
		longMap[uint64(i)] = uint64(i % math.MaxUint16)
	}

	test := LongMapStruct{
		LongMap: longMap,
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.ErrorIs(t, packer.Err(), codec.ErrTooManyItems)
}

func TestMarshalLongString(t *testing.T) {
	type LongStringStruct struct {
		LongString string
	}

	// Create a string that's too long (more than 65535 characters)
	longString := strings.Repeat("a", math.MaxUint16+1)

	test := LongStringStruct{
		LongString: longString,
	}

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(packer, &test)
	require.ErrorIs(t, packer.Err(), codec.ErrStringTooLong)
}
