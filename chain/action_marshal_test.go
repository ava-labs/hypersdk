package chain_test

import (
	"fmt"
	"math"
	reflect "reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
)

func requireNoErrorFast(tb testing.TB, err error) {
	if err != nil {
		tb.Fatal(err)
	}
}

func TestPacksNumbersTighter(t *testing.T) {

}

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

	actualBytes, err := chain.MarshalAction(transfer)
	requireNoErrorFast(t, err)

	require.Equal(t, expectedBytes, actualBytes)

	//unmarshal
	var restoredStruct testStructure
	err = chain.UnmarshalAction(expectedBytes, &restoredStruct)
	requireNoErrorFast(t, err)

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

	bytes, err := chain.MarshalAction(test)
	requireNoErrorFast(t, err)

	var restoredStruct NumberStructure
	err = chain.UnmarshalAction(bytes, &restoredStruct)
	requireNoErrorFast(t, err)

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

	bytes, err := chain.MarshalAction(test)
	requireNoErrorFast(t, err)

	var restoredStruct FlatStructure
	err = chain.UnmarshalAction(bytes, &restoredStruct)
	requireNoErrorFast(t, err)

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

	bytes, err := chain.MarshalAction(test)
	requireNoErrorFast(t, err)

	var restoredStruct FlatStructure
	err = chain.UnmarshalAction(bytes, &restoredStruct)
	requireNoErrorFast(t, err)

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

	bytes, err := chain.MarshalAction(test)
	requireNoErrorFast(t, err)

	var restoredStruct ComplexStruct
	err = chain.UnmarshalAction(bytes, &restoredStruct)
	requireNoErrorFast(t, err)

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

	// Time chain.MarshalAction and chain.UnmarshalAction
	start := time.Now()

	for i := 0; i < iterations; i++ {
		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored TestStruct
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)
	}
	reflectionTime := time.Since(start)

	// Check if reflectionTime is more than 100ms
	if reflectionTime > 100*time.Millisecond {
		t.Fatalf("MarshalAction and UnmarshalAction took too long: %v", reflectionTime)
	}

	t.Logf("Reflection time: %v", reflectionTime)
}

// $ go test -bench=BenchmarkMarshalUnmarshal -benchmem ./chain
// goos: linux
// goarch: amd64
// pkg: github.com/ava-labs/hypersdk/chain
// cpu: AMD EPYC 7763 64-Core Processor
// BenchmarkMarshalUnmarshal/Transfer-Reflection-1-8                     25          43537383 ns/op        35200222 B/op     500002 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Reflection-2-8                     44          27911118 ns/op        35200192 B/op     500003 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Reflection-4-8                     67          18641837 ns/op        35200585 B/op     500006 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Reflection-8-8                     73          15644815 ns/op        35200919 B/op     500011 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-1-8                         85          13315283 ns/op        14400048 B/op     200002 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-2-8                        135           8490382 ns/op        14400117 B/op     200003 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-4-8                        193           6162926 ns/op        14400171 B/op     200005 allocs/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-8-8                        230           5290315 ns/op        14400406 B/op     200009 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-1-8                       8         131688824 ns/op        62400130 B/op    1200002 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-2-8                      14          84294921 ns/op        62400249 B/op    1200003 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-4-8                      20          53936334 ns/op        62400397 B/op    1200005 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Reflection-8-8                      27          42685016 ns/op        62400487 B/op    1200009 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-1-8                          20          56797834 ns/op        40800104 B/op     900002 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-2-8                          38          34366410 ns/op        40800175 B/op     900003 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-4-8                          51          23726648 ns/op        40800266 B/op     900005 allocs/op
// BenchmarkMarshalUnmarshal/Complex-Manual-8-8                          57          18333733 ns/op        40800778 B/op     900010 allocs/op
// PASS
// ok      github.com/ava-labs/hypersdk/chain      22.327s
func BenchmarkMarshalUnmarshal(b *testing.B) {
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
		InnerField: []InnerStruct{
			{
				Field1: 12345,
				Field2: "Inner string",
				Field3: false,
				Field5: []byte{6, 7, 8, 9, 10},
			},
			{
				Field1: 222,
				Field2: "Inner string 2",
				Field3: true,
				Field5: []byte{11, 12, 13, 14, 15},
			},
		},
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
				bytes, err := chain.MarshalAction(transfer)
				requireNoErrorFast(b, err)
				var restored Transfer
				err = chain.UnmarshalAction(bytes, &restored)
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

	for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
		b.Run(fmt.Sprintf("Complex-Reflection-%d", numWorkers), func(b *testing.B) {
			runParallel(b, numWorkers, func() {
				bytes, err := chain.MarshalAction(test)
				requireNoErrorFast(b, err)
				var restored TestStruct
				err = chain.UnmarshalAction(bytes, &restored)
				requireNoErrorFast(b, err)
			})
		})
	}

	for numWorkers := 1; numWorkers <= runtime.NumCPU(); numWorkers *= 2 {
		b.Run(fmt.Sprintf("Complex-Manual-%d", numWorkers), func(b *testing.B) {
			runParallel(b, numWorkers, func() {
				p := codec.NewWriter(0, consts.NetworkSizeLimit)
				p.PackUint64(test.Uint64Field)
				p.PackString(test.StringField)
				p.PackBytes(test.BytesField)
				p.PackInt(test.IntField)
				p.PackBool(test.BoolField)
				p.PackInt(int(test.Uint16Field))
				p.PackInt(int(test.Int8Field))
				p.PackInt(len(test.InnerField))
				for _, inner := range test.InnerField {
					p.PackInt(int(inner.Field1))
					p.PackString(inner.Field2)
					p.PackBool(inner.Field3)
					p.PackBytes(inner.Field5)
				}
				bytes := p.Bytes()

				r := codec.NewReader(bytes, consts.NetworkSizeLimit)
				var restored TestStruct
				restored.Uint64Field = r.UnpackUint64(false)
				restored.StringField = r.UnpackString(false)
				r.UnpackBytes(-1, false, &restored.BytesField)
				restored.IntField = r.UnpackInt(false)
				restored.BoolField = r.UnpackBool()
				restored.Uint16Field = uint16(r.UnpackInt(false))
				restored.Int8Field = int8(r.UnpackInt(false))
				restored.InnerField = make([]InnerStruct, r.UnpackInt(false))
				for i := range restored.InnerField {
					restored.InnerField[i].Field1 = int32(r.UnpackInt(false))
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
			bytes, err := chain.MarshalAction(tt.value)
			requireNoErrorFast(t, err)

			// Check size
			require.Equal(t, tt.expectedSize, len(bytes), "Encoded size mismatch")

			// Decode and compare
			decoded := reflect.New(reflect.TypeOf(tt.value)).Interface()
			err = chain.UnmarshalAction(bytes, decoded)
			requireNoErrorFast(t, err)

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

		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored Outer
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)

		require.Equal(t, test, restored)
	})

	t.Run("EmptyStruct", func(t *testing.T) {
		type Empty struct{}
		test := Empty{}

		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored Empty
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)

		require.Equal(t, test, restored)
	})

	t.Run("UnexportedFields", func(t *testing.T) {
		type Mixed struct {
			Exported   int
			unexported string
		}
		test := Mixed{Exported: 42, unexported: "should be ignored"}

		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored Mixed
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)

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

		_, err := chain.MarshalAction(test)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported field type: ptr")

		bytes := []byte{0x00, 0x01, 0x02} // Some dummy bytes
		var restored PointerStruct
		err = chain.UnmarshalAction(bytes, &restored)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported field type: ptr")
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

		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored Outer
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)

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

		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored Outer
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)

		require.Equal(t, test.OtherField, restored.OtherField)
		require.Zero(t, restored.embedded.field) // Unexported fields should be ignored
	})

	t.Run("StructWithInterface", func(t *testing.T) {
		type Struct struct {
			Field interface{}
		}
		test := Struct{Field: "test"}

		_, err := chain.MarshalAction(test)
		require.Error(t, err) // Should error as interfaces are not supported
	})

	t.Run("CustomType", func(t *testing.T) {
		type CustomInt int
		type Struct struct {
			Field CustomInt
		}
		test := Struct{Field: CustomInt(42)}

		bytes, err := chain.MarshalAction(test)
		requireNoErrorFast(t, err)

		var restored Struct
		err = chain.UnmarshalAction(bytes, &restored)
		requireNoErrorFast(t, err)

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
			bytes, err := chain.MarshalAction(tt.value)
			if err != nil {
				t.Fatalf("MarshalAction failed: %v", err)
			}

			if len(bytes) != tt.expected {
				t.Errorf("Expected length %d, got %d", tt.expected, len(bytes))
			}

			restored := reflect.New(reflect.TypeOf(tt.value)).Interface()
			err = chain.UnmarshalAction(bytes, restored)
			if err != nil {
				t.Fatalf("UnmarshalAction failed: %v", err)
			}

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

	bytes, err := chain.MarshalAction(test)
	if err != nil {
		t.Fatalf("MarshalAction failed: %v", err)
	}

	// Expected length: 70000 bytes for the data + 4 bytes for the length prefix
	expectedLength := 70000 + 4
	if len(bytes) != expectedLength {
		t.Errorf("Expected marshaled length %d, got %d", expectedLength, len(bytes))
	}

	var restored LongBytesStruct
	err = chain.UnmarshalAction(bytes, &restored)
	if err != nil {
		t.Fatalf("UnmarshalAction failed: %v", err)
	}

	if !reflect.DeepEqual(test, restored) {
		t.Errorf("Restored value does not match original")
	}
}
