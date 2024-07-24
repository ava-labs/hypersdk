package chain

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
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

	p := codec.NewWriter(0, consts.NetworkSizeLimit)
	//this is a copy of actions.Transfer.Marshal() logic
	p.PackAddress(transfer.To)
	p.PackUint64(transfer.Value)
	p.PackBytes(transfer.Memo)
	expectedBytes := p.Bytes()

	actualBytes, err := MarshalAction(transfer)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, actualBytes)

	//unmarshal
	var restoredStruct testStructure
	err = UnmarshalAction(expectedBytes, &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, transfer, restoredStruct)
}

func FuzzTestTransfer(f *testing.F) {
	// Add seed corpus
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9}, uint64(12876198273671286), []byte("Hello World"))

	f.Fuzz(func(t *testing.T, toBytes []byte, value uint64, memo []byte) {
		type testStructure struct {
			To    codec.Address `json:"to"`
			Value uint64        `json:"value"`
			Memo  []byte        `json:"memo"`
		}

		// Ensure the To address is valid
		var to codec.Address
		if len(toBytes) > len(to) {
			toBytes = toBytes[:len(to)]
		}
		copy(to[:], toBytes)

		transfer := testStructure{
			To:    to,
			Value: value,
			Memo:  memo,
		}

		// Manual marshaling
		p := codec.NewWriter(0, consts.NetworkSizeLimit)
		p.PackAddress(transfer.To)
		p.PackUint64(transfer.Value)
		p.PackBytes(transfer.Memo)
		expectedBytes := p.Bytes()

		// MarshalAction
		actualBytes, err := MarshalAction(transfer)
		if err != nil {
			t.Fatalf("MarshalAction failed: %v", err)
		}

		if !bytes.Equal(expectedBytes, actualBytes) {
			t.Fatalf("Marshaled bytes do not match. Expected: %v, Got: %v", expectedBytes, actualBytes)
		}

		// UnmarshalAction
		var restoredStruct testStructure
		err = UnmarshalAction(actualBytes, &restoredStruct)
		if err != nil {
			t.Fatalf("UnmarshalAction failed: %v", err)
		}

		if !reflect.DeepEqual(transfer, restoredStruct) {
			t.Fatalf("Restored struct does not match original. Original: %+v, Restored: %+v", transfer, restoredStruct)
		}
	})
}

func TestMarshalNegativeInts(t *testing.T) {
	type NegativeIntStructure struct {
		NegInt   int   `json:"negInt"`
		NegInt8  int8  `json:"negInt8"`
		NegInt16 int16 `json:"negInt16"`
		NegInt32 int32 `json:"negInt32"`
		NegInt64 int64 `json:"negInt64"`
	}

	test := NegativeIntStructure{
		NegInt:   -42,
		NegInt8:  -8,
		NegInt16: -16,
		NegInt32: -32,
		NegInt64: -64,
	}

	bytes, err := MarshalAction(test)
	require.NoError(t, err)

	var restoredStruct NegativeIntStructure
	err = UnmarshalAction(bytes, &restoredStruct)
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

	bytes, err := MarshalAction(test)
	require.NoError(t, err)

	var restoredStruct FlatStructure
	err = UnmarshalAction(bytes, &restoredStruct)
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
		ByteArrayField: []byte{}, //codec would unmarshal nil to []byte{} anyway
	}

	bytes, err := MarshalAction(test)
	require.NoError(t, err)

	var restoredStruct FlatStructure
	err = UnmarshalAction(bytes, &restoredStruct)
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

	bytes, err := MarshalAction(test)
	require.NoError(t, err)

	var restoredStruct ComplexStruct
	err = UnmarshalAction(bytes, &restoredStruct)
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

func TestMakeSureMarshalUnmarshalIsNotTooSlow(t *testing.T) {
	type TestStruct struct {
		Uint64Field uint64
		StringField string
		BytesField  []byte
	}

	test := TestStruct{
		Uint64Field: 42,
		StringField: "Hello, World!",
		BytesField:  []byte{1, 2, 3, 4, 5},
	}

	iterations := 100000

	// Time MarshalAction and UnmarshalAction
	start := time.Now()
	var reflectionBytes []byte
	for i := 0; i < iterations; i++ {
		bytes, err := MarshalAction(test)
		require.NoError(t, err)
		reflectionBytes = bytes

		var restored TestStruct
		err = UnmarshalAction(bytes, &restored)
		require.NoError(t, err)
	}
	reflectionTime := time.Since(start)

	// Time manual packing
	start = time.Now()
	var manualBytes []byte
	for i := 0; i < iterations; i++ {
		p := codec.NewWriter(0, consts.NetworkSizeLimit)
		p.PackUint64(test.Uint64Field)
		p.PackString(test.StringField)
		p.PackBytes(test.BytesField)
		bytes := p.Bytes()
		manualBytes = bytes

		r := codec.NewReader(bytes, consts.NetworkSizeLimit)
		var restored TestStruct
		restored.Uint64Field = r.UnpackUint64(false)
		restored.StringField = r.UnpackString(false)
		r.UnpackBytes(-1, false, &restored.BytesField)
		require.NoError(t, r.Err())
	}
	manualTime := time.Since(start)

	// Compare bytes between the two methods
	require.Equal(t, manualBytes, reflectionBytes, "Bytes from reflection and manual methods differ")

	// Check if reflection is more than 5x as slow
	if float64(reflectionTime) > float64(manualTime)*5 {
		percentage := (float64(reflectionTime)/float64(manualTime) - 1) * 100
		t.Errorf("%d iterations reflection-based marshal/unmarshal is %.2f%% slower than manual packing, takes %v instead of %v", iterations, percentage, reflectionTime, manualTime)
	}
}
func MarshalAction(item interface{}) ([]byte, error) {
	p := codec.NewWriter(0, consts.NetworkSizeLimit) // FIXME: size
	v := reflect.ValueOf(item)
	t := v.Type()

	info := getTypeInfo(t)

	for _, fi := range info {
		field := v.Field(fi.index)
		_, err := marshalValue(p, field, fi.kind, fi.typ)
		if err != nil {
			return nil, err
		}
	}

	return p.Bytes(), nil
}
func FuzzTestMarshalUnmarshal(f *testing.F) {
	// Add seed corpus
	f.Add([]byte("Hello, World!"), uint64(42), int64(-42), int32(1234), uint32(5678))

	f.Fuzz(func(t *testing.T, data []byte, u64 uint64, i64 int64, i32 int32, u32 uint32) {
		type ComplexStruct struct {
			StringField    string        `json:"stringField"`
			Uint64Field    uint64        `json:"uint64Field"`
			Int64Field     int64         `json:"int64Field"`
			Int32Field     int32         `json:"int32Field"`
			Uint32Field    uint32        `json:"uint32Field"`
			ByteArrayField []byte        `json:"byteArrayField"`
			AddressField   codec.Address `json:"addressField"`
			NestedStruct   struct {
				NestedInt    int    `json:"nestedInt"`
				NestedString string `json:"nestedString"`
			} `json:"nestedStruct"`
			SliceField []int           `json:"sliceField"`
			MapField   map[string]bool `json:"mapField"`
		}

		test := ComplexStruct{
			StringField:    string(data),
			Uint64Field:    u64,
			Int64Field:     i64,
			Int32Field:     i32,
			Uint32Field:    u32,
			ByteArrayField: data,
			AddressField:   codec.Address{1, 2, 3}, // TODO: add fuzzing for the address
			NestedStruct: struct {
				NestedInt    int    `json:"nestedInt"`
				NestedString string `json:"nestedString"`
			}{
				NestedInt:    int(i32),
				NestedString: string(data[:min(len(data), 10)]),
			},
			SliceField: []int{int(i32), int(u32)},
			MapField: map[string]bool{
				"key1": u64%2 == 0,
				"key2": i64%2 == 0,
			},
		}

		bytes, err := MarshalAction(test)
		if err != nil {
			t.Fatalf("MarshalAction failed: %v", err)
		}

		var restoredStruct ComplexStruct
		err = UnmarshalAction(bytes, &restoredStruct)
		if err != nil {
			t.Fatalf("UnmarshalAction failed: %v", err)
		}

		if !reflect.DeepEqual(test, restoredStruct) {
			t.Fatalf("Restored struct does not match original. Original: %+v, Restored: %+v", test, restoredStruct)
		}
	})
}

// go test -bench=BenchmarkMarshalUnmarshal -benchmem ./chain
// Reflection time: 91.224215ms
// Manual time: 39.183159ms
// goos: linux
// goarch: amd64
// pkg: github.com/ava-labs/hypersdk/chain
// cpu: AMD EPYC 7763 64-Core Processor
// BenchmarkMarshalUnmarshal/Reflection-4           1335832               900.8 ns/op           168 B/op          6 allocs/op
// BenchmarkMarshalUnmarshal/Manual-4               3042572               396.5 ns/op            72 B/op          4 allocs/op
// PASS
// ok      github.com/ava-labs/hypersdk/chain      3.927s
func BenchmarkMarshalUnmarshal(b *testing.B) {
	type TestStruct struct {
		Uint64Field uint64
		StringField string
		BytesField  []byte
	}

	test := TestStruct{
		Uint64Field: 42,
		StringField: "Hello, World!",
		BytesField:  []byte{1, 2, 3, 4, 5},
	}

	b.Run("Reflection", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bytes, err := MarshalAction(test)
			require.NoError(b, err)
			var restored TestStruct
			err = UnmarshalAction(bytes, &restored)
			require.NoError(b, err)
		}
	})

	b.Run("Manual", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := codec.NewWriter(0, consts.NetworkSizeLimit)
			p.PackUint64(test.Uint64Field)
			p.PackString(test.StringField)
			p.PackBytes(test.BytesField)
			bytes := p.Bytes()

			r := codec.NewReader(bytes, consts.NetworkSizeLimit)
			var restored TestStruct
			restored.Uint64Field = r.UnpackUint64(false)
			restored.StringField = r.UnpackString(false)
			r.UnpackBytes(-1, false, &restored.BytesField)
			require.NoError(b, r.Err())
		}
	})
}
