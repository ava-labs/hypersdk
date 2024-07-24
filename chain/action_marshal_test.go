package chain

import (
	"fmt"
	"reflect"
	"testing"

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

	p := codec.NewWriter(1000, 1000)
	//copy of actions.Transfer.Marshal() logic
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

func MarshalAction(item interface{}) ([]byte, error) {
	p := codec.NewWriter(0, consts.NetworkSizeLimit) // FIXME: size
	v := reflect.ValueOf(item)

	return marshalValue(p, v)
}

func marshalValue(p *codec.Packer, v reflect.Value) ([]byte, error) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			_, err := marshalValue(p, field)
			if err != nil {
				return nil, err
			}
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			if v.IsNil() || v.Len() == 0 {
				p.PackBytes(nil)
			} else {
				p.PackBytes(v.Bytes())
			}
		} else {
			p.PackInt(v.Len())
			for i := 0; i < v.Len(); i++ {
				_, err := marshalValue(p, v.Index(i))
				if err != nil {
					return nil, err
				}
			}
		}
	case reflect.Map:
		p.PackInt(v.Len())
		for _, key := range v.MapKeys() {
			_, err := marshalValue(p, key)
			if err != nil {
				return nil, err
			}
			_, err = marshalValue(p, v.MapIndex(key))
			if err != nil {
				return nil, err
			}
		}
	case reflect.Int:
		p.PackInt64(int64(v.Int()))
	case reflect.Int8:
		p.PackByte(byte(v.Int()))
	case reflect.Int16, reflect.Int32:
		p.PackInt(int(v.Int()))
	case reflect.Int64:
		p.PackInt64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		p.PackInt(int(v.Uint()))
	case reflect.Uint64:
		p.PackUint64(v.Uint())
	case reflect.String:
		p.PackBytes([]byte(v.String()))
	default:
		if v.Type() == reflect.TypeOf(codec.Address{}) {
			if v.Interface().(codec.Address) == codec.EmptyAddress {
				return nil, fmt.Errorf("packer does not support empty addresses")
			}
			p.PackAddress(v.Interface().(codec.Address))
		} else {
			return nil, fmt.Errorf("unsupported field type: %v", v.Kind())
		}
	}

	return p.Bytes(), nil
}

func UnmarshalAction(data []byte, item interface{}) error {
	r := codec.NewReader(data, len(data))
	v := reflect.ValueOf(item).Elem()

	return unmarshalValue(r, v)
}

func unmarshalValue(r *codec.Packer, v reflect.Value) error {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			err := unmarshalValue(r, field)
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			var bytes []byte
			r.UnpackBytes(-1, false, &bytes)
			v.SetBytes(bytes)
		} else {
			length := r.UnpackInt(false)
			slice := reflect.MakeSlice(v.Type(), length, length)
			for i := 0; i < length; i++ {
				err := unmarshalValue(r, slice.Index(i))
				if err != nil {
					return err
				}
			}
			v.Set(slice)
		}
	case reflect.Map:
		length := r.UnpackInt(false)
		m := reflect.MakeMap(v.Type())
		for i := 0; i < length; i++ {
			key := reflect.New(v.Type().Key()).Elem()
			err := unmarshalValue(r, key)
			if err != nil {
				return err
			}
			value := reflect.New(v.Type().Elem()).Elem()
			err = unmarshalValue(r, value)
			if err != nil {
				return err
			}
			m.SetMapIndex(key, value)
		}
		v.Set(m)
	case reflect.Int:
		v.SetInt(r.UnpackInt64(false))
	case reflect.Int8:
		v.SetInt(int64(r.UnpackByte()))
	case reflect.Int16, reflect.Int32:
		v.SetInt(int64(r.UnpackInt(false)))
	case reflect.Int64:
		v.SetInt(r.UnpackInt64(false))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		v.SetUint(uint64(r.UnpackInt(false)))
	case reflect.Uint64:
		v.SetUint(r.UnpackUint64(false))
	case reflect.String:
		var bytes []byte
		r.UnpackBytes(-1, false, &bytes)
		v.SetString(string(bytes))
	default:
		if v.Type() == reflect.TypeOf(codec.Address{}) {
			var addr codec.Address
			r.UnpackAddress(&addr)
			v.Set(reflect.ValueOf(addr))
		} else {
			return fmt.Errorf("unsupported field type: %v", v.Kind())
		}
	}

	if r.Err() != nil {
		return r.Err()
	}

	return nil
}
