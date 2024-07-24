package chain

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ava-labs/hypersdk/codec"
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

	p := codec.NewWriter(1000, 1000)
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
func MarshalAction(item interface{}) ([]byte, error) {
	p := codec.NewWriter(1000, 1000) // FIXME: size
	v := reflect.ValueOf(item)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Int:
			p.PackInt64(int64(field.Int()))
		case reflect.Int8:
			p.PackByte(byte(field.Int()))
		case reflect.Int16, reflect.Int32:
			p.PackInt(int(field.Int()))
		case reflect.Int64:
			p.PackInt64(field.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
			p.PackInt(int(field.Uint()))
		case reflect.Uint64:
			p.PackUint64(field.Uint())
		case reflect.String:
			p.PackBytes([]byte(field.String()))
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 {
				if field.IsNil() || field.Len() == 0 {
					p.PackBytes(nil)
				} else {
					p.PackBytes(field.Bytes())
				}
			} else {
				return nil, fmt.Errorf("unsupported slice element type: %v", field.Kind())
			}
		default:
			if field.Type() == reflect.TypeOf(codec.Address{}) {
				if field.Interface().(codec.Address) == codec.EmptyAddress {
					return nil, fmt.Errorf("packer does not support empty addresses")
				}
				p.PackAddress(field.Interface().(codec.Address))

			} else {
				return nil, fmt.Errorf("unsupported field type: %v", field.Kind())
			}
		}
	}

	return p.Bytes(), nil
}
func UnmarshalAction(data []byte, item interface{}) error {
	r := codec.NewReader(data, len(data))
	v := reflect.ValueOf(item).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Int:
			field.SetInt(r.UnpackInt64(false))
		case reflect.Int8:
			field.SetInt(int64(r.UnpackByte()))
		case reflect.Int16, reflect.Int32:
			field.SetInt(int64(r.UnpackInt(false)))
		case reflect.Int64:
			field.SetInt(r.UnpackInt64(false))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
			field.SetUint(uint64(r.UnpackInt(false)))
		case reflect.Uint64:
			field.SetUint(r.UnpackUint64(false))
		case reflect.String:
			var bytes []byte
			r.UnpackBytes(-1, false, &bytes) // Added support for string
			field.SetString(string(bytes))
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 {
				var bytes []byte
				r.UnpackBytes(-1, false, &bytes)
				field.SetBytes(bytes)
			} else {
				return fmt.Errorf("unsupported slice element type: %v", field.Kind())
			}
		default:
			if field.Type() == reflect.TypeOf(codec.Address{}) {
				var addr codec.Address
				r.UnpackAddress(&addr)
				field.Set(reflect.ValueOf(addr))
			} else {
				return fmt.Errorf("unsupported field type: %v", field.Kind())
			}
		}
		if r.Err() != nil {
			return r.Err()
		}
	}

	return nil
}
