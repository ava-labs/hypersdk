// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dynamic

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

func DynamicMarshal(inputAbi abi.ABI, typeName string, jsonData string) ([]byte, error) {
	// Find the type in the ABI
	abiType := findABIType(inputAbi, typeName)
	if abiType == nil {
		return nil, fmt.Errorf("type %s not found in ABI", typeName)
	}

	// Create a cache to avoid rebuilding types
	typeCache := make(map[string]reflect.Type)

	// Create a dynamic struct type
	dynamicType := getReflectType(typeName, inputAbi, typeCache)

	// Create an instance of the dynamic struct
	dynamicValue := reflect.New(dynamicType).Interface()

	// Unmarshal JSON data into the dynamic struct
	if err := json.Unmarshal([]byte(jsonData), dynamicValue); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	// Marshal the dynamic struct using LinearCodec
	writer := codec.NewWriter(0, consts.NetworkSizeLimit)
	if err := codec.LinearCodec.MarshalInto(dynamicValue, writer.Packer); err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	return writer.Bytes(), nil
}

func DynamicUnmarshal(inputAbi abi.ABI, typeName string, data []byte) (string, error) {
	// Find the type in the ABI
	abiType := findABIType(inputAbi, typeName)
	if abiType == nil {
		return "", fmt.Errorf("type %s not found in ABI", typeName)
	}

	// Create a cache to avoid rebuilding types
	typeCache := make(map[string]reflect.Type)

	// Create a dynamic struct type
	dynamicType := getReflectType(typeName, inputAbi, typeCache)

	// Create an instance of the dynamic struct
	dynamicValue := reflect.New(dynamicType).Interface()

	// Unmarshal the data into the dynamic struct
	if err := codec.LinearCodec.Unmarshal(data, dynamicValue); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Marshal the dynamic struct back to JSON
	jsonData, err := json.Marshal(dynamicValue)
	if err != nil {
		return "", fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	return string(jsonData), nil
}

func getReflectType(abiTypeName string, inputAbi abi.ABI, typeCache map[string]reflect.Type) reflect.Type {
	switch abiTypeName {
	case "string":
		return reflect.TypeOf("")
	case "uint8":
		return reflect.TypeOf(uint8(0))
	case "uint16":
		return reflect.TypeOf(uint16(0))
	case "uint32":
		return reflect.TypeOf(uint32(0))
	case "uint64":
		return reflect.TypeOf(uint64(0))
	case "int8":
		return reflect.TypeOf(int8(0))
	case "int16":
		return reflect.TypeOf(int16(0))
	case "int32":
		return reflect.TypeOf(int32(0))
	case "int64":
		return reflect.TypeOf(int64(0))
	case "Address":
		return reflect.TypeOf(codec.Address{})
	default:
		if strings.HasPrefix(abiTypeName, "[]") {
			elemType := getReflectType(strings.TrimPrefix(abiTypeName, "[]"), inputAbi, typeCache)
			return reflect.SliceOf(elemType)
		} else if strings.HasPrefix(abiTypeName, "[") {
			// Handle fixed-size arrays

			sizeStr := strings.Split(abiTypeName, "]")[0]
			sizeStr = strings.TrimPrefix(sizeStr, "[")

			size, err := strconv.Atoi(sizeStr)
			if err != nil {
				return reflect.TypeOf((*interface{})(nil)).Elem()
			}
			elemType := getReflectType(strings.TrimPrefix(abiTypeName, "["+sizeStr+"]"), inputAbi, typeCache)
			return reflect.ArrayOf(size, elemType)
		}
		// For custom types, recursively construct the struct type

		// Check if type already in cache
		if cachedType, ok := typeCache[abiTypeName]; ok {
			return cachedType
		}

		// Find the type in the ABI
		abiType := findABIType(inputAbi, abiTypeName)
		if abiType == nil {
			// If not found, fallback to interface{}
			return reflect.TypeOf((*interface{})(nil)).Elem()
		}

		// Build fields
		fields := make([]reflect.StructField, len(abiType.Fields))
		for i, field := range abiType.Fields {
			fieldType := getReflectType(field.Type, inputAbi, typeCache)
			fields[i] = reflect.StructField{
				Name: cases.Title(language.English).String(field.Name),
				Type: fieldType,
				Tag:  reflect.StructTag(fmt.Sprintf(`serialize:"true" json:"%s"`, field.Name)),
			}
		}
		// Create struct type
		structType := reflect.StructOf(fields)

		// Cache the type
		typeCache[abiTypeName] = structType

		return structType
	}
}

// Helper function to find ABI type
func findABIType(inputAbi abi.ABI, typeName string) *abi.Type {
	for i := range inputAbi.Types {
		if inputAbi.Types[i].Name == typeName {
			return &inputAbi.Types[i]
		}
	}
	return nil
}
