// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"fmt"
	"go/format"
	"strings"
	"unicode"

	"github.com/ava-labs/avalanchego/utils/set"
)

func GenerateGoStructs(abi ABI, packageName string) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	sb.WriteString("import \"github.com/ava-labs/hypersdk/codec\"\n\n")

	processed := set.Set[string]{}

	for _, typ := range abi.Types {
		if processed.Contains(typ.Name) {
			continue
		}
		processed.Add(typ.Name)

		sb.WriteString(fmt.Sprintf("type %s struct {\n", typ.Name))
		for _, field := range typ.Fields {
			fieldNameUpperCase := strings.ToUpper(field.Name[0:1]) + field.Name[1:]

			// If the first character is uppercase, use the default JSON tag.
			// Otherwise, specify the exported field (upper case) and the lowercase version as the JSON key.
			goType := convertToGoType(field.Type)
			if unicode.IsUpper(rune(field.Name[0])) {
				sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\"`\n", fieldNameUpperCase, goType))
			} else {
				sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\" json:\"%s\"`\n", fieldNameUpperCase, goType, field.Name))
			}
		}
		sb.WriteString("}\n\n")
	}

	for _, action := range abi.Actions {
		sb.WriteString(fmt.Sprintf("func (%s) GetTypeID() uint8 {\n", action.Name))
		sb.WriteString(fmt.Sprintf("\treturn %d\n", action.ID))
		sb.WriteString("}\n\n")
	}

	for _, output := range abi.Outputs {
		sb.WriteString(fmt.Sprintf("func (%s) GetTypeID() uint8 {\n", output.Name))
		sb.WriteString(fmt.Sprintf("\treturn %d\n", output.ID))
		sb.WriteString("}\n\n")
	}

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
	default:
		if strings.HasPrefix(abiType, "[]") {
			return "[]" + convertToGoType(strings.TrimPrefix(abiType, "[]"))
		}
		return abiType // For custom types, we'll use the type name as-is
	}
}
