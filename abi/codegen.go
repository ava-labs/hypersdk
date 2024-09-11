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

func GenerateGoStructs(abi VM, packageName string) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))
	sb.WriteString("import (\n\t\"github.com/ava-labs/hypersdk/codec\"\n)\n\n")

	processed := set.Set[string]{}

	for _, action := range abi.Actions {
		for _, typ := range action.Types {
			if processed.Contains(typ.Name) {
				continue
			}
			processed.Add(typ.Name)

			sb.WriteString(fmt.Sprintf("type %s struct {\n", typ.Name))
			for _, field := range typ.Fields {
				fieldNameUpperCase := strings.ToUpper(field.Name[0:1]) + field.Name[1:]

				goType := convertToGoType(field.Type)
				if unicode.IsUpper(rune(field.Name[0])) {
					sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\"`\n", fieldNameUpperCase, goType))
				} else {
					sb.WriteString(fmt.Sprintf("\t%s %s `serialize:\"true\" json:\"%s\"`\n", fieldNameUpperCase, goType, field.Name))
				}
			}
			sb.WriteString("}\n\n")

			if action.Action == typ.Name {
				sb.WriteString(fmt.Sprintf("func (%s) GetTypeID() uint8 {\n", action.Action))
				sb.WriteString(fmt.Sprintf("\treturn %d\n", action.ID))
				sb.WriteString("}\n")
			}
		}
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
	case "Bytes":
		return "codec.Bytes"
	default:
		if strings.HasPrefix(abiType, "[]") {
			return "[]" + convertToGoType(strings.TrimPrefix(abiType, "[]"))
		}
		return abiType // For custom types, we'll use the type name as-is
	}
}
