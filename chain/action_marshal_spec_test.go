package chain_test

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshalSpec(t *testing.T) {
	type EmbeddedStruct struct {
		EmbeddedField uint64 `json:"embeddedField"`
	}

	tests := []struct {
		name         string
		input        interface{}
		expected     string
		unpackTarget interface{}
	}{
		{
			name: "Simple uint64",
			input: struct {
				Value uint64 `json:"value"`
			}{
				Value: 42,
			},
			expected: "000000000000002a",
			unpackTarget: &struct {
				Value uint64 `json:"value"`
			}{},
		},
		{
			name: "Negative uint32",
			input: struct {
				Value int64 `json:"value"`
			}{
				Value: -12312312,
			},
			expected: "ffffffffff442108",
			unpackTarget: &struct {
				Value int64 `json:"value"`
			}{},
		},
		{
			name: "Mixed types",
			input: struct {
				IntValue    int32         `json:"intValue"`
				StringValue string        `json:"stringValue"`
				Address     codec.Address `json:"address"`
			}{
				IntValue:    -42,
				StringValue: "Hello, World!",
				Address:     codec.Address{1, 2, 3, 4, 5},
			},
			expected: "ffffffd6000d48656c6c6f2c20576f726c6421010203040500000000000000000000000000000000000000000000000000000000",
			unpackTarget: &struct {
				IntValue    int32         `json:"intValue"`
				StringValue string        `json:"stringValue"`
				Address     codec.Address `json:"address"`
			}{},
		},
		{
			name: "Nested structs",
			input: struct {
				Outer struct {
					Inner struct {
						Value int32 `json:"value"`
					} `json:"inner"`
				} `json:"outer"`
			}{
				Outer: struct {
					Inner struct {
						Value int32 `json:"value"`
					} `json:"inner"`
				}{
					Inner: struct {
						Value int32 `json:"value"`
					}{
						Value: 42,
					},
				},
			},
			expected: "0000002a",
			unpackTarget: &struct {
				Outer struct {
					Inner struct {
						Value int32 `json:"value"`
					} `json:"inner"`
				} `json:"outer"`
			}{},
		},
		{
			name: "Embedded structs",
			input: struct {
				EmbeddedStruct
				OwnField string `json:"ownField"`
			}{
				EmbeddedStruct: EmbeddedStruct{EmbeddedField: 42},
				OwnField:       "test",
			},
			expected: "000000000000002a000474657374",
			unpackTarget: &struct {
				EmbeddedStruct
				OwnField string `json:"ownField"`
			}{},
		},
		{
			name: "Various integer sizes",
			input: struct {
				Uint8Field  uint8  `json:"uint8Field"`
				Uint16Field uint16 `json:"uint16Field"`
				Uint32Field uint32 `json:"uint32Field"`
				Uint64Field uint64 `json:"uint64Field"`
			}{
				Uint8Field:  255,
				Uint16Field: 65535,
				Uint32Field: 4294967295,
				Uint64Field: 18446744073709551615,
			},
			expected: "000000ff0000ffffffffffffffffffffffffffff",
			unpackTarget: &struct {
				Uint8Field  uint8  `json:"uint8Field"`
				Uint16Field uint16 `json:"uint16Field"`
				Uint32Field uint32 `json:"uint32Field"`
				Uint64Field uint64 `json:"uint64Field"`
			}{},
		},
		{
			name: "Slice and map",
			input: struct {
				Numbers []uint16          `json:"numbers"`
				Map     map[string]uint32 `json:"map"`
				Flag    bool              `json:"flag"`
			}{
				Numbers: []uint16{1, 2, 3},
				Map:     map[string]uint32{"one": 1, "two": 2},
				Flag:    true,
			},
			expected: "000000030000000100000002000000030000000200036f6e6500000001000374776f0000000201",
			unpackTarget: &struct {
				Numbers []uint16          `json:"numbers"`
				Map     map[string]uint32 `json:"map"`
				Flag    bool              `json:"flag"`
			}{},
		},
		{
			name: "Complex nested structure",
			input: struct {
				ID       uint64           `json:"id"`
				Name     string           `json:"name"`
				Data     []byte           `json:"data"`
				Metadata map[string]int64 `json:"metadata"`
			}{
				ID:       12345,
				Name:     "Test",
				Data:     []byte{0xDE, 0xAD, 0xBE, 0xEF},
				Metadata: map[string]int64{"key1": -10, "key2": 20},
			},
			expected: "000000000000303900045465737400000004deadbeef0000000200046b657931fffffffffffffff600046b6579320000000000000014",
			unpackTarget: &struct {
				ID       uint64           `json:"id"`
				Name     string           `json:"name"`
				Data     []byte           `json:"data"`
				Metadata map[string]int64 `json:"metadata"`
			}{},
		},
		{
			name: "Max and Min values for integer types",
			input: struct {
				MaxUint8  uint8  `json:"maxUint8"`
				MinUint8  uint8  `json:"minUint8"`
				MaxUint16 uint16 `json:"maxUint16"`
				MinUint16 uint16 `json:"minUint16"`
				MaxUint32 uint32 `json:"maxUint32"`
				MinUint32 uint32 `json:"minUint32"`
				MaxUint64 uint64 `json:"maxUint64"`
				MinUint64 uint64 `json:"minUint64"`
				MaxInt8   int8   `json:"maxInt8"`
				MinInt8   int8   `json:"minInt8"`
				MaxInt16  int16  `json:"maxInt16"`
				MinInt16  int16  `json:"minInt16"`
				MaxInt32  int32  `json:"maxInt32"`
				MinInt32  int32  `json:"minInt32"`
				MaxInt64  int64  `json:"maxInt64"`
				MinInt64  int64  `json:"minInt64"`
			}{
				MaxUint8:  255,
				MinUint8:  0,
				MaxUint16: 65535,
				MinUint16: 0,
				MaxUint32: 4294967295,
				MinUint32: 0,
				MaxUint64: 18446744073709551615,
				MinUint64: 0,
				MaxInt8:   127,
				MinInt8:   -128,
				MaxInt16:  32767,
				MinInt16:  -32768,
				MaxInt32:  2147483647,
				MinInt32:  -2147483648,
				MaxInt64:  9223372036854775807,
				MinInt64:  -9223372036854775808,
			},
			expected: "000000ff000000000000ffff00000000ffffffff00000000ffffffffffffffff00000000000000007f8000007fffffff80007fffffff800000007fffffffffffffff8000000000000000",
			unpackTarget: &struct {
				MaxUint8  uint8  `json:"maxUint8"`
				MinUint8  uint8  `json:"minUint8"`
				MaxUint16 uint16 `json:"maxUint16"`
				MinUint16 uint16 `json:"minUint16"`
				MaxUint32 uint32 `json:"maxUint32"`
				MinUint32 uint32 `json:"minUint32"`
				MaxUint64 uint64 `json:"maxUint64"`
				MinUint64 uint64 `json:"minUint64"`
				MaxInt8   int8   `json:"maxInt8"`
				MinInt8   int8   `json:"minInt8"`
				MaxInt16  int16  `json:"maxInt16"`
				MinInt16  int16  `json:"minInt16"`
				MaxInt32  int32  `json:"maxInt32"`
				MinInt32  int32  `json:"minInt32"`
				MaxInt64  int64  `json:"maxInt64"`
				MinInt64  int64  `json:"minInt64"`
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := chain.MarshalAction(tt.input)
			require.NoError(t, err)

			actual := hex.EncodeToString(bytes)
			require.Equal(t, tt.expected, actual, "Marshaled bytes do not match expected hex string")

			err = chain.UnmarshalAction(bytes, tt.unpackTarget)
			require.NoError(t, err)

			require.EqualValues(t, tt.input, reflect.ValueOf(tt.unpackTarget).Elem().Interface())
		})
	}
}
