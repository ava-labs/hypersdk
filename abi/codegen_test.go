package abi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/stretchr/testify/require"
)

func GenerateGoStructs(abi VMABI) string {
	var sb strings.Builder

	sb.WriteString("package generated\n\n")
	sb.WriteString("import (\n\t\"github.com/ava-labs/hypersdk/codec\"\n)\n\n")

	for _, action := range abi.Actions {
		for _, typ := range action.Types {
			sb.WriteString(fmt.Sprintf("type %s struct {\n", typ.Name))
			for _, field := range typ.Fields {
				goType := convertToGoType(field.Type)
				sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\"`\n", field.Name, goType))
			}
			sb.WriteString("}\n\n")

			sb.WriteString(fmt.Sprintf("func (%s) GetTypeID() uint8 {\n", typ.Name))
			sb.WriteString(fmt.Sprintf("\treturn %d\n", action.ID))
			sb.WriteString("}\n")
		}
	}

	return sb.String()
}

func convertToGoType(abiType string) string {
	switch abiType {
	case "string":
		return "string"
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		return abiType
	case "Address":
		return "codec.Address"
	case "StringAsBytes":
		return "codec.StringAsBytes"
	default:
		if strings.HasPrefix(abiType, "[]") {
			return "[]" + convertToGoType(strings.TrimPrefix(abiType, "[]"))
		}
		return abiType // For custom types, we'll use the type name as-is
	}
}

type SimpleStruct struct {
	Field1 string `serialize:"true"`
	Field2 int32  `serialize:"true"`
}

func (SimpleStruct) GetTypeID() uint8 {
	return 1
}

func TestGenerateSimpleStruct(t *testing.T) {
	require := require.New(t)

	abi, err := GetVMABI([]codec.Typed{SimpleStruct{}})
	require.NoError(err)

	code := GenerateGoStructs(abi)
	fmt.Println(code)
	expected := `package generated

import (
	"github.com/ava-labs/hypersdk/codec"
)

type SimpleStruct struct {
	Field1 string ` + "`serialize:\"true\"`" + `
	Field2 int32 ` + "`serialize:\"true\"`" + `
}

func (SimpleStruct) GetTypeID() uint8 {
	return 1
}
`
	require.Equal(expected, code)
}
