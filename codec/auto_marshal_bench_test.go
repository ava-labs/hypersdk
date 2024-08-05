package codec_test

import (
	"bytes"
	"fmt"
	reflect "reflect"
	"testing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/stretchr/testify/require"
)

// $ go test -bench=BenchmarkMarshalUnmarshal -benchmem ./codec
// goos: linux
// goarch: amd64
// pkg: github.com/ava-labs/hypersdk/codec
// cpu: AMD EPYC 7763 64-Core Processor
// BenchmarkMarshalUnmarshal/Transfer-Reflection-8                 18439879                62.80 ns/op
// BenchmarkMarshalUnmarshal/Transfer-Manual-8                     50670040                24.64 ns/op
// BenchmarkMarshalUnmarshal/InnerOuter-Reflection-8                4309082               293.0 ns/op
// BenchmarkMarshalUnmarshal/InnerOuter-Manual-8                   13443223                88.32 ns/op
// BenchmarkMarshalUnmarshal/BigFlatObject-Reflection-8             8192916               137.2 ns/op
// BenchmarkMarshalUnmarshal/BigFlatObject-Manual-8                24067494                46.57 ns/op
// PASS
// ok      github.com/ava-labs/hypersdk/codec      8.315s
func BenchmarkMarshalUnmarshal(b *testing.B) {
	sampleSize := 100000

	type Transfer struct {
		To    codec.Address `json:"to"`
		Value uint64        `json:"value"`
		Memo  []byte        `json:"memo"`
	}
	transfersEncoded := make([][]byte, sampleSize)
	for i := 0; i < sampleSize; i++ {
		transfer := Transfer{
			To:    codec.Address{byte(i % 256), byte((i / 256) % 256), byte((i / 65536) % 256), 4, 5, 6, 7, 8, 9},
			Value: 1000000000000 + uint64(i),
			Memo:  []byte(fmt.Sprintf("Hello World %d", i)),
		}
		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, transfer)
		require.NoError(b, packer.Err())
		transfersEncoded[i] = packer.Bytes()
	}

	unpackAutoTransfer := func(bytes []byte, restored *Transfer) error {
		r := codec.NewReader(bytes, consts.NetworkSizeLimit)
		return codec.AutoUnmarshalStruct(r, restored)
	}

	b.Run("Transfer-Reflection", func(b *testing.B) {
		i := 0

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i++
				var restored Transfer
				err := unpackAutoTransfer(transfersEncoded[i%sampleSize], &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	unpackManualTransfer := func(bytes []byte, restored *Transfer) error {
		r := codec.NewReader(bytes, consts.NetworkSizeLimit)
		r.UnpackAddress(&restored.To)
		restored.Value = r.UnpackUint64(false)
		r.UnpackBytes(-1, false, &restored.Memo)
		return r.Err()
	}

	b.Run("Transfer-Manual", func(b *testing.B) {
		i := 0
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i++
				var restored Transfer
				err := unpackManualTransfer(transfersEncoded[i%sampleSize], &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	//compare auto and manual unmarshalling just to make sure they are the same
	var autoRestored, manualRestored Transfer
	err := unpackAutoTransfer(transfersEncoded[0], &autoRestored)
	require.NoError(b, err)
	err = unpackManualTransfer(transfersEncoded[0], &manualRestored)
	require.NoError(b, err)
	if autoRestored.To != manualRestored.To ||
		autoRestored.Value != manualRestored.Value ||
		!bytes.Equal(autoRestored.Memo, manualRestored.Memo) {
		b.Fatal("mismatch between auto and manual unmarshalled data")
	}

	type InnerStruct struct {
		Field1 int32
		Field2 []byte
	}

	type OuterStruct struct {
		BytesField []byte
		InnerField []InnerStruct
	}

	outerStructsEncoded := make([][]byte, sampleSize)
	for i := 0; i < sampleSize; i++ {

		test := OuterStruct{
			BytesField: []byte("test bytes field"),
			InnerField: func() []InnerStruct {
				innerFields := make([]InnerStruct, 10)
				for j := 0; j < 10; j++ {
					innerFields[j] = InnerStruct{
						Field1: int32(i * (j + 1)),
						Field2: []byte(fmt.Sprintf("inner field %d", i+j)),
					}
				}
				return innerFields
			}(),
		}

		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, test)
		require.NoError(b, packer.Err())
		outerStructsEncoded[i] = packer.Bytes()
	}

	unpackAutoOuter := func(bytes []byte, restored *OuterStruct) error {
		r := codec.NewReader(bytes, consts.NetworkSizeLimit)
		return codec.AutoUnmarshalStruct(r, restored)
	}

	b.Run("InnerOuter-Reflection", func(b *testing.B) {
		i := 0
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i++
				var restored OuterStruct
				err := unpackAutoOuter(outerStructsEncoded[i%sampleSize], &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	unpackManualOuter := func(bytes []byte, restored *OuterStruct) error {
		p := codec.NewReader(bytes, consts.NetworkSizeLimit)
		p.UnpackBytes(-1, false, &restored.BytesField)
		innerLen := p.UnpackShort()
		restored.InnerField = make([]InnerStruct, innerLen)
		for i := uint16(0); i < innerLen; i++ {
			restored.InnerField[i].Field1 = int32(p.UnpackInt(false))
			p.UnpackBytes(-1, false, &restored.InnerField[i].Field2)
		}
		return p.Err()
	}

	b.Run("InnerOuter-Manual", func(b *testing.B) {

		i := 0
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i++
				var restored OuterStruct
				err := unpackManualOuter(outerStructsEncoded[i%sampleSize], &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	//compare auto and manual unmarshalling just to make sure they are the same
	var autoRestoredOuter, manualRestoredOuter OuterStruct
	err = unpackAutoOuter(outerStructsEncoded[0], &autoRestoredOuter)
	require.NoError(b, err)
	err = unpackManualOuter(outerStructsEncoded[0], &manualRestoredOuter)
	require.NoError(b, err)
	if !reflect.DeepEqual(autoRestoredOuter, manualRestoredOuter) {
		b.Fatal("mismatch between auto and manual unmarshalled data")
	}

	type BigFlatObject struct {
		Field1  int32
		Field2  int32
		Field3  int32
		Field4  int32
		Field5  int32
		Field6  int32
		Field7  int32
		Field8  int32
		Field9  int32
		Field10 int32
		Field11 int32
		Field12 int32
		Field13 int32
		Field14 int32
		Field15 int32
		Field16 int32
		Field17 int32
		Field18 int32
		Field19 int32
		Field20 int32
		Field21 int32
		Field22 int32
		Field23 int32
		Field24 int32
		Field25 int32
		Field26 int32
		Field27 int32
		Field28 int32
		Field29 int32
		Field30 int32
	}

	bigFlatObjectsEncoded := make([][]byte, sampleSize)
	for i := 0; i < sampleSize; i++ {
		test := BigFlatObject{
			Field1:  int32(i),
			Field2:  int32(i + 1),
			Field3:  int32(i + 2),
			Field4:  int32(i + 3),
			Field5:  int32(i + 4),
			Field6:  int32(i + 5),
			Field7:  int32(i + 6),
			Field8:  int32(i + 7),
			Field9:  int32(i + 8),
			Field10: int32(i + 9),
			Field11: int32(i + 10),
			Field12: int32(i + 11),
			Field13: int32(i + 12),
			Field14: int32(i + 13),
			Field15: int32(i + 14),
			Field16: int32(i + 15),
			Field17: int32(i + 16),
			Field18: int32(i + 17),
			Field19: int32(i + 18),
			Field20: int32(i + 19),
			Field21: int32(i + 20),
			Field22: int32(i + 21),
			Field23: int32(i + 22),
			Field24: int32(i + 23),
			Field25: int32(i + 24),
			Field26: int32(i + 25),
			Field27: int32(i + 26),
			Field28: int32(i + 27),
			Field29: int32(i + 28),
			Field30: int32(i + 29),
		}
		packer := codec.NewWriter(0, consts.NetworkSizeLimit)
		codec.AutoMarshalStruct(packer, test)
		require.NoError(b, packer.Err())
		bigFlatObjectsEncoded[i] = packer.Bytes()
	}

	unpackAutoBigFlat := func(bytes []byte, restored *BigFlatObject) error {
		r := codec.NewReader(bytes, consts.NetworkSizeLimit)
		return codec.AutoUnmarshalStruct(r, restored)
	}

	b.Run("BigFlatObject-Reflection", func(b *testing.B) {
		i := 0
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i++
				var restored BigFlatObject
				err := unpackAutoBigFlat(bigFlatObjectsEncoded[i%sampleSize], &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	unpackManualBigFlat := func(bytes []byte, restored *BigFlatObject) error {
		p := codec.NewReader(bytes, consts.NetworkSizeLimit)
		restored.Field1 = int32(p.UnpackInt(false))
		restored.Field2 = int32(p.UnpackInt(false))
		restored.Field3 = int32(p.UnpackInt(false))
		restored.Field4 = int32(p.UnpackInt(false))
		restored.Field5 = int32(p.UnpackInt(false))
		restored.Field6 = int32(p.UnpackInt(false))
		restored.Field7 = int32(p.UnpackInt(false))
		restored.Field8 = int32(p.UnpackInt(false))
		restored.Field9 = int32(p.UnpackInt(false))
		restored.Field10 = int32(p.UnpackInt(false))
		restored.Field11 = int32(p.UnpackInt(false))
		restored.Field12 = int32(p.UnpackInt(false))
		restored.Field13 = int32(p.UnpackInt(false))
		restored.Field14 = int32(p.UnpackInt(false))
		restored.Field15 = int32(p.UnpackInt(false))
		restored.Field16 = int32(p.UnpackInt(false))
		restored.Field17 = int32(p.UnpackInt(false))
		restored.Field18 = int32(p.UnpackInt(false))
		restored.Field19 = int32(p.UnpackInt(false))
		restored.Field20 = int32(p.UnpackInt(false))
		restored.Field21 = int32(p.UnpackInt(false))
		restored.Field22 = int32(p.UnpackInt(false))
		restored.Field23 = int32(p.UnpackInt(false))
		restored.Field24 = int32(p.UnpackInt(false))
		restored.Field25 = int32(p.UnpackInt(false))
		restored.Field26 = int32(p.UnpackInt(false))
		restored.Field27 = int32(p.UnpackInt(false))
		restored.Field28 = int32(p.UnpackInt(false))
		restored.Field29 = int32(p.UnpackInt(false))
		restored.Field30 = int32(p.UnpackInt(false))
		return p.Err()
	}

	b.Run("BigFlatObject-Manual", func(b *testing.B) {
		i := 0
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i++
				var restored BigFlatObject
				err := unpackManualBigFlat(bigFlatObjectsEncoded[i%sampleSize], &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	//compare auto and manual unmarshalling just to make sure they are the same
	var autoRestoredBigFlat, manualRestoredBigFlat BigFlatObject
	err = unpackAutoBigFlat(bigFlatObjectsEncoded[0], &autoRestoredBigFlat)
	require.NoError(b, err)
	err = unpackManualBigFlat(bigFlatObjectsEncoded[0], &manualRestoredBigFlat)
	require.NoError(b, err)
	if !reflect.DeepEqual(autoRestoredBigFlat, manualRestoredBigFlat) {
		b.Fatal("mismatch between auto and manual unmarshalled data")
	}
}
