// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import (
	"encoding/json"
	"fmt"
	"strings"

	reflect "reflect"
)

type ABIField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type SingleActionABI struct {
	ID    uint8                 `json:"id"`
	Name  string                `json:"name"`
	Types map[string][]ABIField `json:"types"`
}

type HavingTypeId interface {
	GetTypeID() uint8
}

func GetVmABIString(actions []HavingTypeId) ([]byte, error) {
	vmABI := make([]SingleActionABI, 0)
	for _, action := range actions {
		actionABI, err := getActionABI(action)
		if err != nil {
			return nil, err
		}
		vmABI = append(vmABI, actionABI)
	}
	return json.MarshalIndent(vmABI, "", "  ")
}

func getActionABI(action HavingTypeId) (SingleActionABI, error) {
	t := reflect.TypeOf(action)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	result := SingleActionABI{
		ID:    action.GetTypeID(),
		Name:  t.Name(),
		Types: make(map[string][]ABIField),
	}

	typesleft := []reflect.Type{t}
	typesAlreadyProcessed := make(map[reflect.Type]bool)

	for i := 0; i < 1000; i++ { // curcuit breakers are always good
		var nextType reflect.Type
		nextTypeFound := false
		for _, anotherType := range typesleft {
			if !typesAlreadyProcessed[anotherType] {
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
		typesleft = append(typesleft, moreTypes...)

		typesAlreadyProcessed[nextType] = true
	}

	return result, nil
}

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

		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
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

			for i := 0; i < 100; i++ {
				if fieldType.Kind() == reflect.Slice {
					arrayPrefix += "[]"
					fieldType = fieldType.Elem()
				} else {
					break
				}
			}

			typeName := arrayPrefix + fieldType.Name()

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
