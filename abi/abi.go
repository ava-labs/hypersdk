// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"

	reflect "reflect"
)

type VMABI struct {
	Actions []SingleActionABI `json:"actions"`
}

var _ codec.Typed = (*VMABI)(nil)

const ABI_TYPE_ID = consts.MaxUint8

func (a VMABI) GetTypeID() uint8 {
	return ABI_TYPE_ID
}

func (a *VMABI) Hash() [32]byte {
	writer := codec.NewWriter(0, consts.NetworkSizeLimit)
	err := codec.LinearCodec.MarshalInto(a, writer.Packer)
	if err != nil {
		// should never happen in prod, safe to panic
		panic(fmt.Errorf("failed to marshal abi: %w", err))
	}
	if writer.Err() != nil {
		// should never happen in prod, safe to panic
		panic(fmt.Errorf("failed to marshal abi: %w", writer.Err()))
	}
	abiHash := sha256.Sum256(writer.Bytes())
	return abiHash
}

// ABIField represents a field in the VMABI (Application Binary Interface).
type ABIField struct {
	// Name of the field, overridden by the json tag if present
	Name string `json:"name"`
	// Type of the field, either a Go type or struct name (excluding package name)
	Type string `json:"type"`
}

// SingleActionABI represents the VMABI for an action.
type SingleActionABI struct {
	ID    uint8                 `json:"id"`
	Name  string                `json:"name"`
	Types map[string][]ABIField `json:"types"`
}

// TODO: remove this
func GetVMABIString(actions []codec.Typed) (string, error) {
	vmABI, err := GetVMABI(actions)
	if err != nil {
		return "", err
	}
	resBytes, err := json.Marshal(vmABI)
	return string(resBytes), err
}

func GetVMABI(actions []codec.Typed) (VMABI, error) {
	vmABI := make([]SingleActionABI, 0)
	for _, action := range actions {
		actionABI, err := getActionABI(action)
		if err != nil {
			return VMABI{}, err
		}
		vmABI = append(vmABI, actionABI)
	}
	return VMABI{Actions: vmABI}, nil
}

// getActionABI generates the VMABI for a single action.
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

		serializeTag := field.Tag.Get("serialize")
		if serializeTag != "true" {
			continue
		}

		// Handle JSON tag for field name override
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			fieldName = parts[0]
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

			for fieldType.Name() == "" {
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
			})
		}
	}

	return fields, otherStructsSeen, nil
}
