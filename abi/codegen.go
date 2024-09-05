package abi

import (
	"fmt"
	"go/format"
	"strings"
)

func GenerateGoStructs(abi VMABI) (string, error) {
	var sb strings.Builder

	sb.WriteString("package generated\n\n")
	sb.WriteString("import (\n\t\"github.com/ava-labs/hypersdk/codec\"\n)\n\n")

	for _, action := range abi.Actions {
		for _, typ := range action.Types {
			sb.WriteString(fmt.Sprintf("type %s struct {\n", typ.Name))
			for _, field := range typ.Fields {
				fieldNameUpperCase := strings.ToUpper(field.Name[0:1]) + field.Name[1:]

				goType := convertToGoType(field.Type)
				if field.Name[0] >= 'A' && field.Name[0] <= 'Z' {
					sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\"`\n", fieldNameUpperCase, goType))
				} else {
					sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\" json:\"%s\"`\n", fieldNameUpperCase, goType, field.Name))
				}
			}
			sb.WriteString("}\n\n")

			sb.WriteString(fmt.Sprintf("func (%s) GetTypeID() uint8 {\n", typ.Name))
			sb.WriteString(fmt.Sprintf("\treturn %d\n", action.ID))
			sb.WriteString("}\n")
		}
	}

	fmt.Println(sb.String())
	formatted, err := format.Source([]byte(sb.String()))
	if err != nil {
		return "", fmt.Errorf("failed to format generated code: %w", err)
	}

	return string(formatted), nil
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
