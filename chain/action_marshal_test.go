package chain

import (
	"testing"
	"time"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
)

func TestMarshalUnmarshalAction(t *testing.T) {
	type TestStruct struct {
		Uint8Field uint8 `json:"uint8Field"`
		BoolField  bool  `json:"boolField"`
	}

	test := TestStruct{
		Uint8Field: 8,
		BoolField:  true,
	}

	bytes, err := MarshalAction(test)
	require.NoError(t, err)

	var restoredStruct TestStruct
	err = UnmarshalAction(bytes, &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func TestMarshalUnmarshalEmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	test := EmptyStruct{}

	bytes, err := MarshalAction(test)
	require.NoError(t, err)
	require.Empty(t, bytes)

	var restoredStruct EmptyStruct
	err = UnmarshalAction(bytes, &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func TestMarshalUnmarshalUnsupportedType(t *testing.T) {
	type UnsupportedStruct struct {
		FloatField float64 `json:"floatField"`
	}

	test := UnsupportedStruct{
		FloatField: 3.14,
	}

	_, err := MarshalAction(test)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported field type")
}

func TestMakeSureMarshalUnmarshalIsNotTooSlow(t *testing.T) {
	type TestStruct struct {
		Uint8Field uint8
		BoolField  bool
	}

	test := TestStruct{
		Uint8Field: 42,
		BoolField:  true,
	}

	iterations := 100000

	// Time chain.MarshalAction and chain.UnmarshalAction
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
		p.PackByte(test.Uint8Field)
		p.PackByte(boolToByte(test.BoolField))
		bytes := p.Bytes()
		manualBytes = bytes

		r := codec.NewReader(bytes, consts.NetworkSizeLimit)
		var restored TestStruct
		restored.Uint8Field = r.UnpackByte()
		restored.BoolField = byteToBool(r.UnpackByte())
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

func BenchmarkMarshalUnmarshal(b *testing.B) {
	type TestStruct struct {
		Uint8Field uint8
		BoolField  bool
	}

	test := TestStruct{
		Uint8Field: 42,
		BoolField:  true,
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
			p.PackByte(test.Uint8Field)
			p.PackByte(boolToByte(test.BoolField))
			bytes := p.Bytes()

			r := codec.NewReader(bytes, consts.NetworkSizeLimit)
			var restored TestStruct
			restored.Uint8Field = r.UnpackByte()
			restored.BoolField = byteToBool(r.UnpackByte())
			require.NoError(b, r.Err())
		}
	})
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func byteToBool(b byte) bool {
	return b != 0
}
