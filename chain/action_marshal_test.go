package chain_test

import (
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

	bytes, err := chain.MarshalAction(test)
	requireNoErrorFast(t, err)

	var restoredStruct NegativeIntStructure
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

// go test -bench=BenchmarkMarshalUnmarshal -benchmem ./chain

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
	const numWorkers = 8

	runParallel := func(b *testing.B, f func()) {
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

	b.Run("Reflection-Manual-Complex", func(b *testing.B) {
		runParallel(b, func() {
			bytes, err := chain.MarshalAction(test)
			requireNoErrorFast(b, err)
			var restored TestStruct
			err = chain.UnmarshalAction(bytes, &restored)
			requireNoErrorFast(b, err)
		})
	})

	b.Run("Reflection-Reflection-Complex", func(b *testing.B) {
		runParallel(b, func() {
			bytes, err := chain.MarshalAction(test)
			requireNoErrorFast(b, err)
			var restored TestStruct
			err = chain.UnmarshalAction(bytes, &restored)
			requireNoErrorFast(b, err)
		})
	})

	b.Run("Reflection-Manual-Transfer", func(b *testing.B) {
		runParallel(b, func() {
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

	b.Run("Reflection-Reflection-Transfer", func(b *testing.B) {
		runParallel(b, func() {
			bytes, err := chain.MarshalAction(transfer)
			requireNoErrorFast(b, err)
			var restored Transfer
			err = chain.UnmarshalAction(bytes, &restored)
			requireNoErrorFast(b, err)
		})
	})
}
