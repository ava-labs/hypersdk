// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dynamic

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanchego/utils/wrappers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

var ErrTypeNotFound = errors.New("type not found in ABI")

func Marshal(inputABI abi.ABI, typeName string, jsonData string) ([]byte, error) {
	if _, ok := inputABI.FindTypeByName(typeName); !ok {
		return nil, fmt.Errorf("marshalling %s: %w", typeName, ErrTypeNotFound)
	}

	typeCache := make(map[string]reflect.Type)

	typ, err := getReflectType(typeName, inputABI, typeCache)
	if err != nil {
		return nil, fmt.Errorf("failed to get reflect type: %w", err)
	}

	value := reflect.New(typ).Interface()

	if err := json.Unmarshal([]byte(jsonData), value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	var typeID byte
	found := false
	for _, action := range inputABI.Actions {
		if action.Name == typeName {
			typeID = action.ID
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("action %s not found in ABI", typeName)
	}

	writer := codec.NewWriter(1, consts.NetworkSizeLimit)
	writer.PackByte(typeID)
	if err := codec.LinearCodec.MarshalInto(value, writer.Packer); err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	return writer.Bytes(), nil
}

func UnmarshalOutput(inputABI abi.ABI, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	typeID := data[0]
	outputType, ok := inputABI.FindOutputByID(typeID)
	if !ok {
		return "", fmt.Errorf("output with id %d not found in ABI", typeID)
	}

	return Unmarshal(inputABI, data[1:], outputType.Name)
}

func UnmarshalAction(inputABI abi.ABI, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	typeID := data[0]
	actionType, ok := inputABI.FindActionByID(typeID)
	if !ok {
		return "", fmt.Errorf("action with id %d not found in ABI", typeID)
	}

	return Unmarshal(inputABI, data[1:], actionType.Name)
}

func Unmarshal(inputABI abi.ABI, data []byte, typeName string) (string, error) {
	typeCache := make(map[string]reflect.Type)

	typ, err := getReflectType(typeName, inputABI, typeCache)
	if err != nil {
		return "", fmt.Errorf("failed to get reflect type: %w", err)
	}

	value := reflect.New(typ).Interface()

	packer := wrappers.Packer{
		Bytes:   data,
		MaxSize: consts.NetworkSizeLimit,
	}
	if err := codec.LinearCodec.UnmarshalFrom(&packer, value); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	jsonData, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	return string(jsonData), nil
}

// Matches fixed-size arrays like [32]uint8
var fixedSizeArrayRegex = regexp.MustCompile(`^\[(\d+)\](.+)$`)

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

		if match := fixedSizeArrayRegex.FindStringSubmatch(abiTypeName); match != nil {
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
		if cachedType, ok := typeCache[abiTypeName]; ok {
			return cachedType, nil
		}

		abiType, ok := inputABI.FindTypeByName(abiTypeName)
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
