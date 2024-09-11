// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package abi

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"

	reflect "reflect"
)

type VM struct {
	Actions []Action `serialize:"true" json:"actions"`
}

var _ chain.Typed = (*VM)(nil)

const ABITypeID = consts.MaxUint8

func (VM) GetTypeID() uint8 {
	return ABITypeID
}

func (a *VM) Hash() [32]byte {
	writer := codec.NewWriter(0, consts.NetworkSizeLimit)
	err := codec.LinearCodec.MarshalInto(a, writer.Packer)
	if err != nil {
		// should never happen in prod, safe to panic
		panic(fmt.Errorf("failed to marshal abi: MarshalInto: %w", err))
	}
	if writer.Err() != nil {
		// should never happen in prod, safe to panic
		panic(fmt.Errorf("failed to marshal abi: writer.Err: %w", writer.Err()))
	}
	abiHash := sha256.Sum256(writer.Bytes())
	return abiHash
}

// Field represents a field in the VM (Application Binary Interface).
type Field struct {
	// Name of the field, overridden by the json tag if present
	Name string `serialize:"true" json:"name"`
	// Type of the field, either a Go type or struct name (excluding package name)
	Type string `serialize:"true" json:"type"`
}

// Action represents the VM for an action.
type Action struct {
	ID    uint8  `serialize:"true" json:"id"`
	Name  string `serialize:"true" json:"name"`
	Types []Type `serialize:"true" json:"types"`
}

type Type struct {
	Name   string  `serialize:"true" json:"name"`
	Fields []Field `serialize:"true" json:"fields"`
}

func DescribeVM(actions []chain.Typed) (VM, error) {
	vmABI := make([]Action, 0)
	for _, action := range actions {
		actionABI, err := describeAction(action)
		if err != nil {
			return VM{}, err
		}
		vmABI = append(vmABI, actionABI)
	}
	return VM{Actions: vmABI}, nil
}

// describeAction generates the VM for a single action.
// It handles both struct and pointer types, and recursively processes nested structs.
// Does not support maps or interfaces - only standard go types, slices, arrays and structs
func describeAction(action chain.Typed) (Action, error) {
	t := reflect.TypeOf(action)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	result := Action{
		ID:    action.GetTypeID(),
		Name:  t.Name(),
		Types: make([]Type, 0),
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
			return Action{}, err
		}

		result.Types = append(result.Types, Type{
			Name:   nextType.Name(),
			Fields: fields,
		})
		typesLeft = append(typesLeft, moreTypes...)

		typesAlreadyProcessed.Add(nextType)
	}

	return result, nil
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
