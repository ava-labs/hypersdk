package chain

import (
	"fmt"
	"reflect"
	"strconv"
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

func TestMarshalMapStringToAddress(t *testing.T) {
	type testStructure struct {
		Addresses map[string]codec.Address `json:"addresses"`
	}

	test := testStructure{
		Addresses: map[string]codec.Address{
			"alice": {1, 2, 3, 4, 5, 6, 7, 8, 9},
			"bob":   {9, 8, 7, 6, 5, 4, 3, 2, 1},
		},
	}

	p := codec.NewWriter(1000, 1000)
	p.PackInt((len(test.Addresses)))
	for k, v := range test.Addresses {
		p.PackString(k)
		p.PackAddress(v)
	}
	expectedBytes := p.Bytes()

	actualBytes, err := MarshalAction(test)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, actualBytes)

	var restoredStruct testStructure
	err = UnmarshalAction(expectedBytes, &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func TestMarshalComplexStructure(t *testing.T) {
	type ComplexStructure struct {
		Int   int   `json:"int"`
		Int8  int8  `json:"int8"`
		Int16 int16 `json:"int16"`
		Int32 int32 `json:"int32"`
		Int64 int64 `json:"int64"`
		// Uint         uint                     `json:"uint"`
		// Uint8        uint8                    `json:"uint8"`
		// Uint16       uint16                   `json:"uint16"`
		// Uint32       uint32                   `json:"uint32"`
		// Uint64       uint64                   `json:"uint64"`
		// Float32      float32                  `json:"float32"`
		// Float64      float64                  `json:"float64"`
		// String       string                   `json:"string"`
		// ByteArray    []byte                   `json:"byteArray"`
		// Address      codec.Address            `json:"address"`
		// IntArray     []int                    `json:"intArray"`
		// StringArray  []string                 `json:"stringArray"`
		// AddressArray []codec.Address          `json:"addressArray"`
		// IntMap       map[string]int           `json:"intMap"`
		// StringMap    map[string]string        `json:"stringMap"`
		// AddressMap   map[string]codec.Address `json:"addressMap"`
	}

	test := ComplexStructure{
		Int:   42,
		Int8:  8,
		Int16: 16,
		Int32: 32,
		Int64: 64,
		// Uint:        42,
		// Uint8:       8,
		// Uint16:      16,
		// Uint32:      32,
		// Uint64:      64,
		// Float32:     3.14,
		// Float64:     3.14159265359,
		// String:      "test string",
		// ByteArray:   []byte{1, 2, 3, 4, 5},
		// Address:     codec.Address{1, 2, 3, 4, 5, 6, 7, 8, 9},
		// IntArray:    []int{1, 2, 3, 4, 5},
		// StringArray: []string{"one", "two", "three"},
		// AddressArray: []codec.Address{
		// 	{1, 2, 3, 4, 5, 6, 7, 8, 9},
		// 	{9, 8, 7, 6, 5, 4, 3, 2, 1},
		// },
		// IntMap: map[string]int{
		// 	"one": 1,
		// 	"two": 2,
		// },
		// StringMap: map[string]string{
		// 	"key1": "value1",
		// 	"key2": "value2",
		// },
		// AddressMap: map[string]codec.Address{
		// 	"alice": {1, 2, 3, 4, 5, 6, 7, 8, 9},
		// 	"bob":   {9, 8, 7, 6, 5, 4, 3, 2, 1},
		// },
	}

	p := codec.NewWriter(1000, 1000)
	p.PackInt(test.Int)
	p.PackByte(byte(test.Int8))
	p.PackInt(int(test.Int16))
	p.PackInt(int(test.Int32))
	p.PackInt64(test.Int64)
	// p.PackInt(int(test.Uint))
	// p.PackByte(byte(test.Uint8))
	// p.PackInt(int(test.Uint16))
	// p.PackInt(int(test.Uint32))
	// p.PackUint64(test.Uint64)
	// p.PackBytes([]byte(strconv.FormatFloat(float64(test.Float32), 'f', -1, 32)))
	// p.PackBytes([]byte(strconv.FormatFloat(test.Float64, 'f', -1, 64)))
	// p.PackString(test.String)
	// p.PackBytes(test.ByteArray)
	// p.PackAddress(test.Address)
	// p.PackInt(len(test.IntArray))
	// for _, v := range test.IntArray {
	// 	p.PackInt(v)
	// }
	// p.PackInt(len(test.StringArray))
	// for _, v := range test.StringArray {
	// 	p.PackString(v)
	// }
	// p.PackInt(len(test.AddressArray))
	// for _, v := range test.AddressArray {
	// 	p.PackAddress(v)
	// }
	// p.PackInt(len(test.IntMap))
	// for k, v := range test.IntMap {
	// 	p.PackString(k)
	// 	p.PackInt(v)
	// }
	// p.PackInt(len(test.StringMap))
	// for k, v := range test.StringMap {
	// 	p.PackString(k)
	// 	p.PackString(v)
	// }
	// p.PackInt(len(test.AddressMap))
	// for k, v := range test.AddressMap {
	// 	p.PackString(k)
	// 	p.PackAddress(v)
	// }
	expectedBytes := p.Bytes()

	actualBytes, err := MarshalAction(test)
	require.NoError(t, err)

	require.Equal(t, expectedBytes, actualBytes)

	var restoredStruct ComplexStructure
	err = UnmarshalAction(expectedBytes, &restoredStruct)
	require.NoError(t, err)

	require.Equal(t, test, restoredStruct)
}

func MarshalAction(item interface{}) ([]byte, error) {
	p := codec.NewWriter(1000, 1000) // FIXME: size
	v := reflect.ValueOf(item)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fmt.Println(field.Kind())
		fmt.Println(field.Type())
		fmt.Println(field.Interface())
		switch field.Kind() {
		case reflect.Int, reflect.Int16, reflect.Int32:
			p.PackInt(int(field.Int()))
		case reflect.Int8:
			p.PackByte(byte(field.Int()))
		case reflect.Int64:
			p.PackInt64(field.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
			p.PackInt(int(field.Uint()))
		case reflect.Uint64:
			p.PackUint64(field.Uint())
		case reflect.Float32:
			p.PackBytes([]byte(strconv.FormatFloat(float64(field.Float()), 'f', -1, 32)))
		case reflect.Float64:
			p.PackBytes([]byte(strconv.FormatFloat(field.Float(), 'f', -1, 64)))
		case reflect.String:
			p.PackString(field.String())
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 {
				p.PackBytes(field.Bytes())
			} else {
				p.PackInt(field.Len())
				for j := 0; j < field.Len(); j++ {
					elem := field.Index(j)
					switch elem.Kind() {
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
						p.PackInt(int(elem.Int()))
					case reflect.Int64:
						p.PackInt64(elem.Int())
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
						p.PackInt(int(elem.Uint()))
					case reflect.Uint64:
						p.PackUint64(elem.Uint())
					case reflect.String:
						p.PackString(elem.String())
					case reflect.Array:
						if elem.Type() == reflect.TypeOf(codec.Address{}) {
							p.PackAddress(elem.Interface().(codec.Address))
						} else {
							return nil, fmt.Errorf("unsupported array type: %v", elem.Type())
						}
					default:
						return nil, fmt.Errorf("unsupported slice element type: %v", elem.Kind())
					}
				}
			}
		case reflect.Array:
			if field.Type() == reflect.TypeOf(codec.Address{}) {
				p.PackAddress(field.Interface().(codec.Address))
			} else {
				return nil, fmt.Errorf("unsupported array type: %v", field.Type())
			}
		case reflect.Map:
			p.PackInt(field.Len())
			for _, key := range field.MapKeys() {
				p.PackString(key.String())
				value := field.MapIndex(key)
				switch value.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
					p.PackInt(int(value.Int()))
				case reflect.Int64:
					p.PackInt64(value.Int())
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
					p.PackInt(int(value.Uint()))
				case reflect.Uint64:
					p.PackUint64(value.Uint())
				case reflect.String:
					p.PackString(value.String())
				case reflect.Array:
					if value.Type() == reflect.TypeOf(codec.Address{}) {
						p.PackAddress(value.Interface().(codec.Address))
					} else {
						return nil, fmt.Errorf("unsupported map value array type: %v", value.Type())
					}
				default:
					return nil, fmt.Errorf("unsupported map value type: %v", value.Kind())
				}
			}
		default:
			return nil, fmt.Errorf("unsupported field type: %v", field.Kind())
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
		case reflect.Int8:
			field.SetInt(int64(r.UnpackByte()))
		case reflect.Int, reflect.Int16, reflect.Int32:
			field.SetInt(int64(r.UnpackInt(true)))
		case reflect.Int64:
			field.SetInt(r.UnpackInt64(true))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
			field.SetUint(uint64(r.UnpackInt(true)))
		case reflect.Uint64:
			field.SetUint(r.UnpackUint64(true))
		case reflect.Float32:
			var bytes []byte
			r.UnpackBytes(-1, true, &bytes)
			f, err := strconv.ParseFloat(string(bytes), 32)
			if err != nil {
				return fmt.Errorf("failed to unpack float32: %w", err)
			}
			field.SetFloat(f)
		case reflect.Float64:
			var bytes []byte
			r.UnpackBytes(-1, true, &bytes)
			f, err := strconv.ParseFloat(string(bytes), 64)
			if err != nil {
				return fmt.Errorf("failed to unpack float64: %w", err)
			}
			field.SetFloat(f)
		case reflect.String:
			field.SetString(r.UnpackString(true))
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 {
				var bytes []byte
				r.UnpackBytes(-1, true, &bytes)
				field.SetBytes(bytes)
			} else {
				length := r.UnpackInt(true)
				slice := reflect.MakeSlice(field.Type(), length, length)
				for j := 0; j < length; j++ {
					elem := slice.Index(j)
					switch elem.Kind() {
					case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
						elem.SetInt(int64(r.UnpackInt(true)))
					case reflect.Int64:
						elem.SetInt(r.UnpackInt64(true))
					case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
						elem.SetUint(uint64(r.UnpackInt(true)))
					case reflect.Uint64:
						elem.SetUint(r.UnpackUint64(true))
					case reflect.String:
						elem.SetString(r.UnpackString(true))
					case reflect.Array:
						if elem.Type() == reflect.TypeOf(codec.Address{}) {
							var addr codec.Address
							r.UnpackAddress(&addr)
							elem.Set(reflect.ValueOf(addr))
						} else {
							return fmt.Errorf("unsupported slice element array type: %v", elem.Type())
						}
					default:
						return fmt.Errorf("unsupported slice element type: %v", elem.Kind())
					}
				}
				field.Set(slice)
			}
		case reflect.Array:
			if field.Type() == reflect.TypeOf(codec.Address{}) {
				var addr codec.Address
				r.UnpackAddress(&addr)
				field.Set(reflect.ValueOf(addr))
			} else {
				return fmt.Errorf("unsupported array type: %v", field.Type())
			}
		case reflect.Map:
			length := r.UnpackInt(true)
			m := reflect.MakeMap(field.Type())
			for j := 0; j < length; j++ {
				key := reflect.New(field.Type().Key()).Elem()
				key.SetString(r.UnpackString(true))
				value := reflect.New(field.Type().Elem()).Elem()
				switch value.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
					value.SetInt(int64(r.UnpackInt(true)))
				case reflect.Int64:
					value.SetInt(r.UnpackInt64(true))
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
					value.SetUint(uint64(r.UnpackInt(true)))
				case reflect.Uint64:
					value.SetUint(r.UnpackUint64(true))
				case reflect.String:
					value.SetString(r.UnpackString(true))
				case reflect.Array:
					if value.Type() == reflect.TypeOf(codec.Address{}) {
						var addr codec.Address
						r.UnpackAddress(&addr)
						value.Set(reflect.ValueOf(addr))
					} else {
						return fmt.Errorf("unsupported map value array type: %v", value.Type())
					}
				default:
					return fmt.Errorf("unsupported map value type: %v", value.Kind())
				}
				m.SetMapIndex(key, value)
			}
			field.Set(m)
		default:
			return fmt.Errorf("unsupported field type: %v", field.Kind())
		}
		if r.Err() != nil {
			return r.Err()
		}
	}

	return nil
}
