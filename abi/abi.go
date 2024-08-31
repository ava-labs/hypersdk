// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"encoding/json"
	"fmt"
	"strings"

	reflect "reflect"

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
)

// ABIField represents a field in the ABI (Application Binary Interface).
type ABIField struct {
	// Name of the field, overridden by the json tag if present
	Name string `json:"name"`
	// Type of the field, either a Go type or struct name (excluding package name)
	Type string `json:"type"`
	// Mask is an optional field used to mask the field's value.
	// Currently, only "byteString" is supported as a mask for []byte values.
	// In general, it would take any string and pass it to ABI, please refer to the frontend wallet specs for more details
	Mask string `json:"mask,omitempty"`
}

// SingleActionABI represents the ABI for an action.
type SingleActionABI struct {
	ID    uint8                 `json:"id"`
	Name  string                `json:"name"`
	Types map[string][]ABIField `json:"types"`
}

func GetVMABIString(actions []codec.Typed) (string, error) {
	vmABI, err := getVMABI(actions)
	if err != nil {
		return "", err
	}
	resBytes, err := json.Marshal(vmABI)
	return string(resBytes), err
}

func getVMABI(actions []codec.Typed) ([]SingleActionABI, error) {
	vmABI := make([]SingleActionABI, 0)
	for _, action := range actions {
		actionABI, err := getActionABI(action)
		if err != nil {
			return nil, err
		}
		vmABI = append(vmABI, actionABI)
	}
	return vmABI, nil
}

// getActionABI generates the ABI for a single action.
// It handles both struct and pointer types, and recursively processes nested structs.
// Does not support maps or interfaces - only standard go types, slices, arrays and structs
func getActionABI(action codec.Typed) (SingleActionABI, error) {
	t := reflect.TypeOf(action)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	result := SingleActionABI{
		ID:    action.GetTypeID(),
		Name:  t.Name(),
		Types: make(map[string][]ABIField),
	}

	typesLeft := []reflect.Type{t}
	typesAlreadyProcessed := set.Set[reflect.Type]{}

	// Process all types, including nested ones
	for {
		var nextType reflect.Type
		nextTypeFound := false
		for _, anotherType := range typesLeft {
			if !typesAlreadyProcessed.Contains(anotherType) {
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
			return SingleActionABI{}, err
		}

		result.Types[nextType.Name()] = fields
		typesLeft = append(typesLeft, moreTypes...)

		typesAlreadyProcessed.Add(nextType)
	}

	return result, nil
}

// describeStruct analyzes a struct type and returns its fields and any nested struct types it found
func describeStruct(t reflect.Type) ([]ABIField, []reflect.Type, error) {
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
		mask := ""

		// Handle JSON tag for field name override
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			fieldName = parts[0]
		}

		// Handle mask tag
		maskTag := field.Tag.Get("mask")
		if maskTag != "" {
			mask = maskTag
		}

		if field.Anonymous && fieldType.Kind() == reflect.Struct {
			// Handle embedded struct by flattening its fields
			embeddedFields, moreTypes, err := describeStruct(fieldType)
			if err != nil {
				return nil, nil, err
			}
			fields = append(fields, embeddedFields...)
			otherStructsSeen = append(otherStructsSeen, moreTypes...)
		} else {
			arrayPrefix := ""

			// Handle nested slices (up to 100 levels deep)
			for i := 0; i < 100; i++ {
				if fieldType.Kind() != reflect.Slice {
					break
				}
				arrayPrefix += "[]"
				fieldType = fieldType.Elem()
			}

			typeName := arrayPrefix + fieldType.Name()

			// Add nested structs and pointers to structs to the list for processing
			if fieldType.Kind() == reflect.Struct {
				otherStructsSeen = append(otherStructsSeen, fieldType)
			} else if fieldType.Kind() == reflect.Ptr {
				otherStructsSeen = append(otherStructsSeen, fieldType.Elem())
			}

			fields = append(fields, ABIField{
				Name: fieldName,
				Type: typeName,
				Mask: mask,
			})
		}
	}

	return fields, otherStructsSeen, nil
}
