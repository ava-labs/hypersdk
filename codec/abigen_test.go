package codec

import (
	"encoding/json"
	reflect "reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type ABINode struct {
	Type       string             `json:"type"`
	Properties map[string]ABINode `json:"properties,omitempty"`
	Items      *ABINode           `json:"items,omitempty"`
}

func GetABIString(t reflect.Type) ([]byte, error) {
	abiNode, err := GetABINode(t)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(abiNode, "", "  ")
}

func GetABINode(t reflect.Type) (ABINode, error) {
	abiNode := ABINode{
		Type: t.String(), // Include package name
	}

	kind := t.Kind()

	prefix := ""
	if kind == reflect.Slice {
		prefix = "[]"
		t = t.Elem()
	}

	if kind == reflect.Struct {
		abiNode.Properties = make(map[string]ABINode)

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			fieldType := field.Type
			fieldName := field.Name

			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				fieldName = parts[0]
			}

			prop, err := GetABINode(fieldType)
			if err != nil {
				return ABINode{}, err
			}

			abiNode.Properties[fieldName] = prop
		}
	} else {
		abiNode.Type = prefix + t.String()
	}

	return abiNode, nil
}

func TestGetABI(t *testing.T) {
	require := require.New(t)

	type Struct1 struct {
		Field1 string
		Field2 int
	}

	abiString, err := GetABIString(reflect.TypeOf(Struct1{}))
	require.NoError(err)
	require.JSONEq(`
		{
			"type": "codec.Struct1",
			"properties": {
				"Field1": {
					"type": "string"
				},
				"Field2": {
					"type": "int"
				}
			}
		}
	`, string(abiString))

	type Transfer struct {
		// To is the recipient of the [Value].
		To Address `json:"to"`

		// Amount are transferred to [To].
		Value uint64 `json:"value"`

		// Optional message to accompany transaction.
		Memo []byte `json:"memo,omitempty"`
	}

	abiString, err = GetABIString(reflect.TypeOf(Transfer{}))
	require.NoError(err)

	require.JSONEq(`
		{
			"type": "codec.Transfer",
			"properties": {
				"to": {
					"type": "codec.Address"
				},
				"value": {
					"type": "uint64"
				},
				"memo": {
					"type": "[]uint8"
				}
			}
		}
	`, string(abiString))

	type AllInts struct {
		Int8   int8
		Int16  int16
		Int32  int32
		Int64  int64
		Uint8  uint8
		Uint16 uint16
		Uint32 uint32
		Uint64 uint64
	}

	abiString, err = GetABIString(reflect.TypeOf(AllInts{}))
	require.NoError(err)

	require.JSONEq(`
		{
			"type": "codec.AllInts",
			"properties": {
				"Int8": {
					"type": "int8"
				},
				"Int16": {
					"type": "int16"
				},
				"Int32": {
					"type": "int32"
				},
				"Int64": {
					"type": "int64"
				},
				"Uint8": {
					"type": "uint8"
				},
				"Uint16": {
					"type": "uint16"
				},
				"Uint32": {
					"type": "uint32"
				},
				"Uint64": {
					"type": "uint64"
				}
			}
		}
	`, string(abiString))

	type InnerStruct struct {
		Field1 string
		Field2 int
	}

	type OuterStruct struct {
		SingleItem InnerStruct   `json:"single_item"`
		ArrayItems []InnerStruct `json:"array_items"`
	}

	abiString, err = GetABIString(reflect.TypeOf(OuterStruct{}))
	require.NoError(err)

	require.Equal(`
		{
			"type": "codec.OuterStruct",
			"properties": {
				"SingleItem": {
					"type": "codec.InnerStruct"
				},
				"ArrayItems": {
					"type": "[]codec.InnerStruct"
				}
			}
		}
		`, string(abiString))
}
