// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dynamic

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

// Matches fixed-size arrays like [32]uint8
var fixedSizeArrayRegex = regexp.MustCompile(`^\[(\d+)\](.+)$`)

func Marshal(inputABI abi.ABI, typeName string, jsonData string) ([]byte, error) {
	_, ok := findABIType(inputABI, typeName)
	if !ok {
		return nil, fmt.Errorf("type %s not found in ABI", typeName)
	}

	typeCache := make(map[string]reflect.Type)

	typ, err := getReflectType(typeName, inputABI, typeCache)
	if err != nil {
		return nil, fmt.Errorf("failed to get reflect type: %w", err)
	}

	value := reflect.New(typ).Interface()

	err = json.Unmarshal([]byte(jsonData), value)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	writer := codec.NewWriter(0, consts.NetworkSizeLimit)
	err = codec.LinearCodec.MarshalInto(value, writer.Packer)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	return writer.Bytes(), nil
}

func Unmarshal(inputABI abi.ABI, typeName string, data []byte) (string, error) {
	_, ok := findABIType(inputABI, typeName)
	if !ok {
		return "", fmt.Errorf("type %s not found in ABI", typeName)
	}

	typeCache := make(map[string]reflect.Type)

	dynamicType, err := getReflectType(typeName, inputABI, typeCache)
	if err != nil {
		return "", fmt.Errorf("failed to get reflect type: %w", err)
	}

	dynamicValue := reflect.New(dynamicType).Interface()

	err = codec.LinearCodec.Unmarshal(data, dynamicValue)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	jsonData, err := json.Marshal(dynamicValue)
	if err != nil {
		return "", fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	return string(jsonData), nil
}

func getReflectType(abiTypeName string, inputABI abi.ABI, typeCache map[string]reflect.Type) (reflect.Type, error) {
	switch abiTypeName {
	case "string":
		return reflect.TypeOf(""), nil
	case "uint8":
		return reflect.TypeOf(uint8(0)), nil
	case "uint16":
		return reflect.TypeOf(uint16(0)), nil
	case "uint32":
		return reflect.TypeOf(uint32(0)), nil
	case "uint64":
		return reflect.TypeOf(uint64(0)), nil
	case "int8":
		return reflect.TypeOf(int8(0)), nil
	case "int16":
		return reflect.TypeOf(int16(0)), nil
	case "int32":
		return reflect.TypeOf(int32(0)), nil
	case "int64":
		return reflect.TypeOf(int64(0)), nil
	case "Address":
		return reflect.TypeOf(codec.Address{}), nil
	default:
		// golang slices
		if strings.HasPrefix(abiTypeName, "[]") {
			elemType, err := getReflectType(strings.TrimPrefix(abiTypeName, "[]"), inputABI, typeCache)
			if err != nil {
				return nil, err
			}
			return reflect.SliceOf(elemType), nil
		}

		// golang arrays
		match := fixedSizeArrayRegex.FindStringSubmatch(abiTypeName) // ^\[(\d+)\](.+)$
		if match != nil {
			sizeStr := match[1]
			size, err := strconv.Atoi(sizeStr)
			if err != nil {
				return nil, fmt.Errorf("failed to convert size to int: %w", err)
			}
			elemType, err := getReflectType(match[2], inputABI, typeCache)
			if err != nil {
				return nil, err
			}
			return reflect.ArrayOf(size, elemType), nil
		}

		// For custom types, recursively construct the struct type
		cachedType, ok := typeCache[abiTypeName]
		if ok {
			return cachedType, nil
		}

		abiType, ok := findABIType(inputABI, abiTypeName)
		if !ok {
			return nil, fmt.Errorf("type %s not found in ABI", abiTypeName)
		}

		// It is a struct, as we don't support anything else as custom types
		fields := make([]reflect.StructField, len(abiType.Fields))
		for i, field := range abiType.Fields {
			fieldType, err := getReflectType(field.Type, inputABI, typeCache)
			if err != nil {
				return nil, err
			}
			fields[i] = reflect.StructField{
				Name: cases.Title(language.English).String(field.Name),
				Type: fieldType,
				Tag:  reflect.StructTag(fmt.Sprintf(`serialize:"true" json:"%s"`, field.Name)),
			}
		}

		structType := reflect.StructOf(fields)
		typeCache[abiTypeName] = structType

		return structType, nil
	}
}

func findABIType(inputABI abi.ABI, typeName string) (abi.Type, bool) {
	for _, typ := range inputABI.Types {
		if typ.Name == typeName {
			return typ, true
		}
	}
	return abi.Type{}, false
}
