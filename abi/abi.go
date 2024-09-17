// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/codec"
)

type ABI struct {
	Actions []Action `serialize:"true" json:"actions"`
	Types   []Type   `serialize:"true" json:"types"`
}

var _ codec.Typed = (*ABI)(nil)

func (ABI) GetTypeID() uint8 {
	return 0
}

type Field struct {
	Name string `serialize:"true" json:"name"`
	Type string `serialize:"true" json:"type"`
}

type Action struct {
	ID     uint8  `serialize:"true" json:"id"`
	Action string `serialize:"true" json:"action"`
}

type Type struct {
	Name   string  `serialize:"true" json:"name"`
	Fields []Field `serialize:"true" json:"fields"`
}

func NewABI(actions []codec.Typed) (ABI, error) {
	vmActions := make([]Action, 0)
	vmTypes := make([]Type, 0)
	typesSet := set.Set[string]{}
	typesAlreadyProcessed := set.Set[reflect.Type]{}

	for _, action := range actions {
		actionABI, typeABI, err := describeAction(action, typesAlreadyProcessed)
		if err != nil {
			return ABI{}, err
		}
		vmActions = append(vmActions, actionABI)
		for _, t := range typeABI {
			if !typesSet.Contains(t.Name) {
				vmTypes = append(vmTypes, t)
				typesSet.Add(t.Name)
			}
		}
	}
	return ABI{Actions: vmActions, Types: vmTypes}, nil
}

// describeAction generates the Action and Types for a single action.
// It handles both struct and pointer types, and recursively processes nested structs.
// Does not support maps or interfaces - only standard go types, slices, arrays and structs
func describeAction(action codec.Typed, typesAlreadyProcessed set.Set[reflect.Type]) (Action, []Type, error) {
	t := reflect.TypeOf(action)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	actionABI := Action{
		ID:     action.GetTypeID(),
		Action: t.Name(),
	}

	typesABI := make([]Type, 0)
	typesLeft := []reflect.Type{t}

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
			return Action{}, nil, err
		}

		typesABI = append(typesABI, Type{
			Name:   nextType.Name(),
			Fields: fields,
		})
		typesLeft = append(typesLeft, moreTypes...)

		typesAlreadyProcessed.Add(nextType)
	}

	return actionABI, typesABI, nil
}

// describeStruct analyzes a struct type and returns its fields and any nested struct types it found
func describeStruct(t reflect.Type) ([]Field, []reflect.Type, error) {
	kind := t.Kind()

	if kind != reflect.Struct {
		return nil, nil, fmt.Errorf("type %s is not a struct", t)
	}

	fields := make([]Field, 0)
	otherStructsSeen := make([]reflect.Type, 0)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type
		fieldName := field.Name

		// Skip any field that will not be serialized by the codec
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

			// Here we assume that all types without a name are slices.
			// We completely ignore the fact that maps exist as we don't support them.
			// Types like `type Bytes = []byte` are slices technically, but they have a name
			// and we need them to be named types instead of slices.
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

			fields = append(fields, Field{
				Name: fieldName,
				Type: typeName,
			})
		}
	}

	return fields, otherStructsSeen, nil
}
