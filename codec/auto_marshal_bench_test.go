package codec_test

import (
	"fmt"
	"testing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func BenchmarkMarshalUnmarshal(b *testing.B) {
	type InnerStruct struct {
		Field1 int32
		Field2 string
	}

	type TestStruct struct {
		StringField string
		BytesField  []byte
		InnerField  []InnerStruct
	}

	test := TestStruct{
		StringField: "Hello, World!",
		BytesField:  []byte{1, 2, 3, 4, 5},
		InnerField: func() []InnerStruct {
			inner := make([]InnerStruct, 100)
			for i := 0; i < 100; i++ {
				inner[i] = InnerStruct{
					Field1: int32(i),
					Field2: fmt.Sprintf("Inner string %d", i),
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
	_ = transfer

	b.Run("Transfer-Reflection", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				packer := codec.NewWriter(0, consts.NetworkSizeLimit)
				codec.AutoMarshalStruct(packer, transfer)
				if err := packer.Err(); err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
				var restored Transfer
				err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	b.Run("Transfer-Manual", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
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
				if err := r.Err(); err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	b.Run("Complex-Reflection", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				packer := codec.NewWriter(0, consts.NetworkSizeLimit)
				codec.AutoMarshalStruct(packer, &test)
				if err := packer.Err(); err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
				var restored TestStruct
				err := codec.AutoUnmarshalStruct(codec.NewReader(packer.Bytes(), consts.NetworkSizeLimit), &restored)
				if err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})

	b.Run("Complex-Manual", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				p := codec.NewWriter(0, consts.NetworkSizeLimit*100)
				p.PackString(test.StringField)
				p.PackBytes(test.BytesField)
				p.PackInt(uint32(len(test.InnerField)))
				for _, inner := range test.InnerField {
					p.PackInt(uint32(inner.Field1))
					p.PackString(inner.Field2)
				}
				bytes := p.Bytes()

				r := codec.NewReader(bytes, len(bytes))
				var restored TestStruct
				restored.StringField = r.UnpackString(false)
				r.UnpackBytes(-1, false, &restored.BytesField)
				restored.InnerField = make([]InnerStruct, r.UnpackInt(false))
				for i := range restored.InnerField {
					restored.InnerField[i].Field1 = int32(r.UnpackInt(false))
					restored.InnerField[i].Field2 = r.UnpackString(false)
				}
				if err := r.Err(); err != nil {
					b.Fatal(err) //nolint:forbidigo
				}
			}
		})
	})
}
