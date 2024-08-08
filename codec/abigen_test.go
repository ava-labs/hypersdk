package codec

import (
	"encoding/json"
	"fmt"
	reflect "reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type ABIField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func GetABIString(t reflect.Type) ([]byte, error) {
	var result map[string][]ABIField = make(map[string][]ABIField)

	typesleft := []reflect.Type{t}
	typesAlreadyProcessed := make(map[reflect.Type]bool)

	for i := 0; i < 100; i++ { //curcuit breaker
		var nextType reflect.Type
		nextTypeFound := false
		for _, anotherType := range typesleft {
			if !typesAlreadyProcessed[anotherType] {
				nextType = anotherType
				nextTypeFound = true
				break
			}
		}
		if !nextTypeFound {
			break
		}

		fields, moreTypes, err := describeStruct(nextType)
		if err != nil {
			return nil, err
		}

		result[nextType.Name()] = fields
		typesleft = append(typesleft, moreTypes...)

		typesAlreadyProcessed[nextType] = true
	}

	return json.MarshalIndent(result, "", "  ")
}

func describeStruct(t reflect.Type) ([]ABIField, []reflect.Type, error) { //reflect.Type returns other types to describe
	kind := t.Kind()

	if kind != reflect.Struct {
		return nil, nil, fmt.Errorf("type %s is not a struct", t.String())
	}

	fields := make([]ABIField, 0)
	otherStructsSeen := make([]reflect.Type, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type
		fieldName := field.Name

		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			fieldName = parts[0]
		}

		typeName := fieldType.Name()
		if fieldType.Kind() == reflect.Slice {
			typeName = "[]" + fieldType.Elem().Name()

			if fieldType.Elem().Kind() == reflect.Struct {
				otherStructsSeen = append(otherStructsSeen, fieldType.Elem())
			}
		} else if fieldType.Kind() == reflect.Ptr {
			otherStructsSeen = append(otherStructsSeen, fieldType.Elem())
		}

		fields = append(fields, ABIField{
			Name: fieldName,
			Type: typeName,
		})
	}

	return fields, otherStructsSeen, nil
}

func TestGetABI(t *testing.T) {
	require := require.New(t)

	type Struct1 struct {
		Field1 string
		Field2 int32
	}

	abiString, err := GetABIString(reflect.TypeOf(Struct1{}))
	require.NoError(err)
	require.JSONEq(`
		{
				"Struct1": [
					{ "name": "Field1", "type": "string" },
					{ "name": "Field2", "type": "int32" }
			]
		}
	`, string(abiString))

	type Transfer struct {
		To    Address `json:"to"`
		Value uint64  `json:"value"`
		Memo  []byte  `json:"memo,omitempty"`
	}

	abiString, err = GetABIString(reflect.TypeOf(Transfer{}))
	require.NoError(err)

	require.JSONEq(`
		{
				"Transfer": [
					{ "name": "to", "type": "Address" },
					{ "name": "value", "type": "uint64" },
					{ "name": "memo", "type": "[]uint8" }
				]
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
				"AllInts": [
					{ "name": "Int8", "type": "int8" },
					{ "name": "Int16", "type": "int16" },
					{ "name": "Int32", "type": "int32" },
					{ "name": "Int64", "type": "int64" },
					{ "name": "Uint8", "type": "uint8" },
					{ "name": "Uint16", "type": "uint16" },
					{ "name": "Uint32", "type": "uint32" },
					{ "name": "Uint64", "type": "uint64" }
				]
		}
	`, string(abiString))

	type InnerStruct struct {
		Field1 string
		Field2 uint64
	}

	type OuterStruct struct {
		SingleItem InnerStruct   `json:"single_item"`
		ArrayItems []InnerStruct `json:"array_items"`
	}

	abiString, err = GetABIString(reflect.TypeOf(OuterStruct{}))
	require.NoError(err)

	require.JSONEq(`
		{
				"OuterStruct": [
					{ "name": "single_item", "type": "InnerStruct" },
					{ "name": "array_items", "type": "[]InnerStruct" }
				],
				"InnerStruct": [
					{ "name": "Field1", "type": "string" },
					{ "name": "Field2", "type": "uint64" }
				]
		}
	`, string(abiString))
}
