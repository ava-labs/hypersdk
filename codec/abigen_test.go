package codec

import (
	"encoding/json"
	"fmt"
	reflect "reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func GetABI(t reflect.Type) (string, error) {
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("expected struct, got %v", t.Kind())
	}

	structName := t.Name()
	fields := make(map[string]string)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.IsExported() {
			fieldType := field.Type.String()
			if fieldType == "[]uint8" {
				fieldType = "[]byte"
			}
			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				fieldName = parts[0]
			}
			fields[fieldName] = fieldType
		}
	}

	abi := []map[string]interface{}{
		{
			"name":   structName,
			"fields": fields,
		},
	}

	abiJSON, err := json.Marshal(abi)
	if err != nil {
		return "", err
	}

	return string(abiJSON), nil
}
func TestGetABI(t *testing.T) {
	require := require.New(t)

	type Struct1 struct {
		Field1 string
		Field2 int
	}

	abiString, err := GetABI(reflect.TypeOf(Struct1{}))
	require.NoError(err)

	require.JSONEq(`
	[
		{
			"name": "Struct1",
			"fields": {
				"Field1": "string",
				"Field2": "int"
			}
		}
	]`, abiString)

	type Transfer struct {
		// To is the recipient of the [Value].
		To Address `json:"to"`

		// Amount are transferred to [To].
		Value uint64 `json:"value"`

		// Optional message to accompany transaction.
		Memo []byte `json:"memo,omitempty"`
	}

	abiString, err = GetABI(reflect.TypeOf(Transfer{}))
	require.NoError(err)

	require.JSONEq(`
	[
		{
			"name": "Transfer",
			"fields": {
				"to": "codec.Address",
				"value": "uint64",
				"memo": "[]byte"
			}
		}
	]`, abiString)

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

	abiString, err = GetABI(reflect.TypeOf(AllInts{}))
	require.NoError(err)

	require.JSONEq(`
	[
		{
			"name": "AllInts",
			"fields": {
				"Int8": "int8",
				"Int16": "int16",
				"Int32": "int32",
				"Int64": "int64",
				"Uint8": "uint8",
				"Uint16": "uint16",
				"Uint32": "uint32",
				"Uint64": "uint64"
			}
		}
	]`, abiString)
}
