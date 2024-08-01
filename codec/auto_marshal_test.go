package codec_test

import (
	"fmt"
	"math"
	reflect "reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

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

	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	//this is a copy of actions.Transfer.Marshal() logic
	packer.PackAddress(transfer.To)
	packer.PackUint64(transfer.Value)
	packer.PackBytes(transfer.Memo)
	expectedBytes := packer.Bytes()

	packer = codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(transfer, packer)
	require.NoError(t, packer.Err())

	require.Equal(t, expectedBytes, packer.Bytes())

	//unmarshal
	var restoredStruct testStructure
	err := codec.AutoUnmarshalStruct(codec.NewReader(expectedBytes, len(expectedBytes)), &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, transfer, restoredStruct)
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
	codec.AutoMarshalStruct(test, packer)
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
	codec.AutoMarshalStruct(test, packer)
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
		ByteArrayField: []byte{}, //codec would unmarshal nil to []byte{} anyway
	}
	packer := codec.NewWriter(0, consts.NetworkSizeLimit)
	codec.AutoMarshalStruct(test, packer)
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
	codec.AutoMarshalStruct(test, packer)
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

	iterations := 100_000

	// Time codec.AutoMarshalStruct and codec.AutoUnmarshalStruct
	start := time.Now()

	for i := 0; i < iterations; i++ {
		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(test, packer)
		if packer.Err() != nil {
			t.Fatal(packer.Err())
		}

		var restored TestStruct
		err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
		if err != nil {
			t.Fatal(err)
		}
	}
	reflectionTime := time.Since(start)

	// Check if reflectionTime is more than 100ms
	if reflectionTime > 100*time.Millisecond {
		t.Fatalf("MarshalAction and UnmarshalAction took too long: %v", reflectionTime)
	}

	t.Logf("Reflection time: %v", reflectionTime)
}

// $ go test -bench=BenchmarkMarshalUnmarshal -benchmem ./codec
// goos: linux
// goarch: amd64
// pkg: github.com/ava-labs/hypersdk/codec
// cpu: AMD EPYC 7763 64-Core Processor
// BenchmarkMarshalUnmarshal/Transfer-Reflection-1-8                     25          43809594 ns/op        35200210 B/op     500002 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Reflection-2-8                     43          27888527 ns/op        35200225 B/op     500003 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Reflection-4-8                     61          18754757 ns/op        35200542 B/op     500006 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Reflection-8-8                     74          15720307 ns/op        35201033 B/op     500011 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-1-8                         82          13465238 ns/op        14400048 B/op     200002 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-2-8                        144           8436937 ns/op        14400132 B/op     200003 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-4-8                        190           6117171 ns/op        14400180 B/op     200005 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-8-8                        232           5136082 ns/op        14400428 B/op     200009 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-1-8                       1        2922380605 ns/op        2051203920 B/op 11700021 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-2-8                       1        1828922789 ns/op        2051205248 B/op 11700003 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-4-8                       1        1212280807 ns/op        2051207296 B/op 11700019 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-8-8                       1        1067666547 ns/op        2051209072 B/op 11700045 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-1-8                           1        1306538593 ns/op        2029602864 B/op 11400002 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-2-8                           2         796309518 ns/op        2029603760 B/op 11400003 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-4-8                           2         624686344 ns/op        2029612464 B/op 11400075 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-8-8                           2         535399140 ns/op        2029608216 B/op 11400034 allocs/op
// PASS
// ok      github.com/ava-labs/hypersdk/codec      25.784s
func BenchmarkMarshalUnmarshal(b *testing.B) {
	requireNoErrorFast := func(tb testing.TB, err error) {
		if err != nil {
			tb.Fatal(err)
		}
	}
	type InnerStruct struct {
		Field1 int32
		Field2 string
		Field3 bool
		Field5 []byte
	}

	type TestStruct struct {
		Uint64Field uint64
		StringField string
		BytesField  []byte
		IntField    int
		BoolField   bool
		Uint16Field uint16
		Int8Field   int8
		InnerField  []InnerStruct
	}

	test := TestStruct{
		Uint64Field: 42,
		StringField: "Hello, World!",
		BytesField:  []byte{1, 2, 3, 4, 5},
		IntField:    -100,
		BoolField:   true,
		Uint16Field: 65535,
		Int8Field:   -128,
		InnerField: func() []InnerStruct {
			inner := make([]InnerStruct, 100)
			for i := 0; i < 100; i++ {
				inner[i] = InnerStruct{
					Field1: int32(i),
					Field2: fmt.Sprintf("Inner string %d", i),
					Field3: i%2 == 0,
					Field5: []byte{byte(i), byte(i + 1), byte(i + 2), byte(i + 3), byte(i + 4)},
				}
			}
			return inner
		}(),
	}

	type Transfer struct {
		To    codec.Address `json:"to"`
		Value uint64        `json:"value"`
		Memo  []byte        `json:"memo"`
	}

	transfer := Transfer{
		To:    codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Value: 12876198273671286,
		Memo:  []byte("Hello World"),
	}

	const iterationsInBatch = 100_000

	// type megabyteStruct struct {
	// 	Field1 []byte
	// }

	// megabyte := megabyteStruct{
	// 	Field1: func() []byte {
	// 		data := make([]byte, 1024*1024)
	// 		for i := range data {
	// 			data[i] = byte(i % math.MaxUint8)
	// 		}
	// 		return data
	// 	}(),
	// }

	runParallel := func(b *testing.B, numWorkers int, f func()) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			wg.Add(numWorkers)
			for w := 0; w < numWorkers; w++ {
				go func() {
					defer wg.Done()
					for j := 0; j < iterationsInBatch/numWorkers; j++ {
						f()
					}
				}()
			}
			wg.Wait()
		}
	}

	for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
		b.Run(fmt.Sprintf("Transfer-Reflection-%d", numWorkers), func(b *testing.B) {
			runParallel(b, numWorkers, func() {
				packer := codec.NewWriter(0, consts.NetworkSizeLimit)
				codec.AutoMarshalStruct(transfer, packer)
				requireNoErrorFast(b, packer.Err())
				var restored Transfer
				err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
				requireNoErrorFast(b, err)
			})
		})
	}

	for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
		b.Run(fmt.Sprintf("Transfer-Manual-%d", numWorkers), func(b *testing.B) {
			runParallel(b, numWorkers, func() {
				p := codec.NewWriter(0, consts.NetworkSizeLimit)
				p.PackAddress(transfer.To)
				p.PackUint64(transfer.Value)
				p.PackBytes(transfer.Memo)
				bytes := p.Bytes()

				r := codec.NewReader(bytes, consts.NetworkSizeLimit)
				var restored Transfer
				r.UnpackAddress(&restored.To)
				restored.Value = r.UnpackUint64(false)
				r.UnpackBytes(-1, false, &restored.Memo)
				requireNoErrorFast(b, r.Err())
			})
		})
	}

	// for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
	// 	b.Run(fmt.Sprintf("Megabyte-Reflection-%d", numWorkers), func(b *testing.B) {
	// 		runParallel(b, numWorkers, func() {
	// 			bytes, err := codec.AutoMarshalStruct(megabyte)
	// 			requireNoErrorFast(b, err)
	// 			var restored megabyteStruct
	// 			err = codec.AutoUnmarshalStruct(bytes, &restored)
	// 			requireNoErrorFast(b, err)
	// 		})
	// 	})
	// }

	// for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
	// 	b.Run(fmt.Sprintf("Megabyte-Manual-%d", numWorkers), func(b *testing.B) {
	// 		runParallel(b, numWorkers, func() {
	// 			p := codec.NewWriter(0, consts.NetworkSizeLimit)
	// 			p.PackBytes(megabyte.Field1)
	// 			bytes := p.Bytes()

	// 			r := codec.NewReader(bytes, consts.NetworkSizeLimit)
	// 			var restored megabyteStruct
	// 			r.UnpackBytes(-1, false, &restored.Field1)
	// 			requireNoErrorFast(b, r.Err())
	// 		})
	// 	})
	// }

	for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
		b.Run(fmt.Sprintf("Complex-Reflection-%d", numWorkers), func(b *testing.B) {
			runParallel(b, numWorkers, func() {
				packer := codec.NewWriter(0, consts.NetworkSizeLimit)
				codec.AutoMarshalStruct(test, packer)
				requireNoErrorFast(b, packer.Err())
				var restored TestStruct
				err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
				requireNoErrorFast(b, err)
			})
		})
	}

	for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
		b.Run(fmt.Sprintf("Complex-Manual-%d", numWorkers), func(b *testing.B) {
			runParallel(b, numWorkers, func() {
				p := codec.NewWriter(0, consts.NetworkSizeLimit*100)
				p.PackUint64(test.Uint64Field)
				p.PackString(test.StringField)
				p.PackBytes(test.BytesField)
				p.PackInt64(int64(test.IntField))
				p.PackBool(test.BoolField)
				p.PackInt(uint32(test.Uint16Field))
				p.PackInt(uint32(test.Int8Field))
				p.PackInt(uint32(len(test.InnerField)))
				for _, inner := range test.InnerField {
					p.PackInt64(int64(inner.Field1))
					p.PackString(inner.Field2)
					p.PackBool(inner.Field3)
					p.PackBytes(inner.Field5)
				}
				bytes := p.Bytes()

				r := codec.NewReader(bytes, len(bytes))
				var restored TestStruct
				restored.Uint64Field = r.UnpackUint64(false)
				restored.StringField = r.UnpackString(false)
				r.UnpackBytes(-1, false, &restored.BytesField)
				restored.IntField = int(r.UnpackInt64(false))
				restored.BoolField = r.UnpackBool()
				restored.Uint16Field = uint16(r.UnpackInt64(false))
				restored.Int8Field = int8(r.UnpackInt64(false))
				restored.InnerField = make([]InnerStruct, r.UnpackInt(false))
				for i := range restored.InnerField {
					restored.InnerField[i].Field1 = int32(r.UnpackInt64(false))
					restored.InnerField[i].Field2 = r.UnpackString(false)
					restored.InnerField[i].Field3 = r.UnpackBool()
					r.UnpackBytes(-1, false, &restored.InnerField[i].Field5)
				}
				requireNoErrorFast(b, r.Err())
			})
		})

	}
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
			codec.AutoMarshalStruct(tt.value, packer)
			require.NoError(t, packer.Err())
			bytes := packer.Bytes()

			// Check size
			require.Equal(t, tt.expectedSize, len(bytes), "Encoded size mismatch")

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
		codec.AutoMarshalStruct(test, packer)
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
		codec.AutoMarshalStruct(test, packer)
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
		codec.AutoMarshalStruct(test, packer)
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
		codec.AutoMarshalStruct(test, packer)
		require.Error(t, packer.Err())
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
		codec.AutoMarshalStruct(&test, packer)
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
		codec.AutoMarshalStruct(test, packer)
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
		codec.AutoMarshalStruct(test, packer)
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
		codec.AutoMarshalStruct(test, packer)
		require.Error(t, packer.Err())
	})

	t.Run("CustomType", func(t *testing.T) {
		type CustomInt int
		type Struct struct {
			Field CustomInt
		}
		test := Struct{Field: CustomInt(42)}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(test, packer)
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
			codec.AutoMarshalStruct(tt.value, packer)
			require.NoError(t, packer.Err())
			bytes := packer.Bytes()

			if len(bytes) != tt.expected {
				t.Errorf("Expected length %d, got %d", tt.expected, len(bytes))
			}

			restored := reflect.New(reflect.TypeOf(tt.value)).Interface()
			err := codec.AutoUnmarshalStruct(codec.NewReader(bytes, consts.NetworkSizeLimit), restored)
			require.NoError(t, err)

			if !reflect.DeepEqual(tt.value, reflect.ValueOf(restored).Elem().Interface()) {
				t.Errorf("Value mismatch. Expected %v, got %v", tt.value, reflect.ValueOf(restored).Elem().Interface())
			}
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
	codec.AutoMarshalStruct(test, packer)
	require.NoError(t, packer.Err())
	bytes := packer.Bytes()

	// 70000 bytes for the data + 4 bytes for the length prefix
	expectedLength := 70000 + 4
	if len(bytes) != expectedLength {
		t.Errorf("Expected marshaled length %d, got %d", expectedLength, len(bytes))
	}

	var restored LongBytesStruct
	err := codec.AutoUnmarshalStruct(codec.NewReader(bytes, consts.NetworkSizeLimit), &restored)
	if err != nil {
		t.Fatalf("UnmarshalAction failed: %v", err)
	}

	if !reflect.DeepEqual(test, restored) {
		t.Errorf("Restored value does not match original")
	}
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
	codec.AutoMarshalStruct(test, packer)
	require.Error(t, packer.Err())
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
	codec.AutoMarshalStruct(test, packer)
	require.Error(t, packer.Err())
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
	codec.AutoMarshalStruct(test, packer)
	require.Error(t, packer.Err())
}

func TestIntIs64Bit(t *testing.T) {
	var i int
	var i64 int64

	require.Equal(t, unsafe.Sizeof(i), unsafe.Sizeof(i64), "int is not 64-bit on this system")
}
