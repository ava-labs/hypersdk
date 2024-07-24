package chain

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

// test structure

// this is a test, don't change it
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

// this is implementation, fix it
func MarshalAction(item interface{}) ([]byte, error) {
	p := codec.NewWriter(1000, 1000) // FIXME: size
	v := reflect.ValueOf(item)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Array:
			if field.Type() == reflect.TypeOf(codec.Address{}) {
				if len(field.Interface().(codec.Address)) == 0 {
					return nil, fmt.Errorf("can not pack empty address")
				}
				p.PackAddress(field.Interface().(codec.Address))
			}
		case reflect.Uint64:
			p.PackUint64(field.Interface().(uint64))
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 {
				p.PackBytes(field.Bytes())
			}
		}
	}

	return p.Bytes(), nil
}

// original example below
// func UnmarshalTransfer(p *codec.Packer) (chain.Action, error) {
// 	var transfer Transfer
// 	p.UnpackAddress(&transfer.To) // we do not verify the typeID is valid
// 	transfer.Value = p.UnpackUint64(true)
// 	p.UnpackBytes(MaxMemoSize, false, &transfer.Memo)
// 	return &transfer, p.Err()
// }

func UnmarshalAction(data []byte, item interface{}) error {
	r := codec.NewReader(data, len(data))
	v := reflect.ValueOf(item).Elem()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fmt.Println(field.Kind())
		switch field.Kind() {
		case reflect.Array:
			if field.Type() == reflect.TypeOf(codec.Address{}) {
				var addr codec.Address
				r.UnpackAddress(&addr)
				if r.Err() != nil {
					return r.Err()
				}
				field.Set(reflect.ValueOf(addr))
			}
		case reflect.Uint64:
			val := r.UnpackUint64(true)
			if r.Err() != nil {
				return r.Err()
			}
			field.SetUint(val)
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 {
				var bytes []byte
				r.UnpackBytes(999999, false, &bytes)
				if r.Err() != nil {
					return r.Err()
				}
				field.SetBytes(bytes)
			}
		}
	}

	return nil
}
