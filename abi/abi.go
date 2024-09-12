// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/chain"
)

type ABI struct {
	Actions []Action `serialize:"true" json:"actions"`
	Types   []Type   `serialize:"true" json:"types"`
}

var _ chain.Typed = (*ABI)(nil)

func (ABI) GetTypeID() uint8 {
	return 0
}

// Field represents a field in the VM (Application Binary Interface).
type Field struct {
	// Name of the field, overridden by the json tag if present
	Name string `serialize:"true" json:"name"`
	// Type of the field, either a Go type or struct name (excluding package name)
	Type string `serialize:"true" json:"type"`
}

// Action represents an action in the VM.
type Action struct {
	ID     uint8  `serialize:"true" json:"id"`
	Action string `serialize:"true" json:"action"`
	Output string `serialize:"true" json:"output"`
}

type Type struct {
	Name   string  `serialize:"true" json:"name"`
	Fields []Field `serialize:"true" json:"fields"`
}

func NewABI(actions []chain.ActionPair) (ABI, error) {
	vmActions := make([]Action, 0)
	vmTypes := make([]Type, 0)
	typesSet := set.Set[string]{}

	for _, action := range actions {
		actionABI, typeABI, err := describeAction(action)
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
func describeAction(action chain.ActionPair) (Action, []Type, error) {
	t := reflect.TypeOf(action.Input)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	actionABI := Action{
		ID:     action.Input.GetTypeID(),
		Action: t.Name(),
	}

	typesABI := make([]Type, 0)
	typesLeft := []reflect.Type{t}
	typesAlreadyProcessed := set.Set[reflect.Type]{}

	if action.Output != nil {
		outputType := reflect.TypeOf(action.Output)
		if outputType.Kind() == reflect.Ptr {
			outputType = outputType.Elem()
		}
		actionABI.Output = outputType.Name()
		typesLeft = append(typesLeft, outputType)
	}

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
		return nil, nil, fmt.Errorf("type %s is not a struct", t.String())
	}

	fields := make([]Field, 0)
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

			fields = append(fields, Field{
				Name: fieldName,
				Type: typeName,
			})
		}
	}

	return fields, otherStructsSeen, nil
}
