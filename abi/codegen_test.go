package abi

import (
	"encoding/json"
	"go/format"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateSimpleStruct(t *testing.T) {
	require := require.New(t)

	abi := mustJSONParse[VMABI](t, `
{
    "actions": [
        {
            "id": 1,
            "name": "SampleObj",
            "types": [
                {
                    "name": "SampleObj",
                    "fields": [
                        {
                            "name": "Field1",
                            "type": "uint16"
                        },
                        {
                            "name": "lowercaseField",
                            "type": "string"
                        },
                        {
                            "name": "a",
                            "type": "string"
                        }
                    ]
                }
            ]
        }
    ]
}`)

	code, err := GenerateGoStructs(abi)
	require.NoError(err)

	expected := `
package generated

import (
	"github.com/ava-labs/hypersdk/codec"
)

type SampleObj struct {
	Field1 uint16 ` + "`serialize:\"true\"`" + `
	LowercaseField string ` + "`serialize:\"true\" json:\"lowercaseField\"`" + `
	A string ` + "`serialize:\"true\" json:\"a\"`" + `
}

func (SampleObj) GetTypeID() uint8 {
	return 1
}
`

	require.Equal(mustFormat(t, expected), code)
}

func mustFormat(t *testing.T, code string) string {
	formatted, err := format.Source([]byte(code))
	require.NoError(t, err)

	return string(formatted)
}

func mustJSONParse[T any](t *testing.T, jsonStr string) T {
	var parsed T
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err, jsonStr)
	return parsed
}
