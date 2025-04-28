// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/StephenButtolph/canoto"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/codec"
)

func fillAction(cmd *cobra.Command, spec *canoto.Spec) (canoto.Any, error) {
	// get key-value pairs
	inputData, err := cmd.Flags().GetStringToString("data")
	if err != nil {
		return canoto.Any{}, fmt.Errorf("failed to get data key-value pairs: %w", err)
	}

	isJSONOutput, err := isJSONOutputRequested(cmd)
	if err != nil {
		return canoto.Any{}, fmt.Errorf("failed to get output format: %w", err)
	}

	isInteractive := len(inputData) == 0 && !isJSONOutput

	var kvPairs canoto.Any
	if isInteractive {
		kvPairs, err = askForFlags(spec)
		if err != nil {
			return canoto.Any{}, fmt.Errorf("failed to ask for flags: %w", err)
		}
	} else {
		kvPairs, err = fillFromInputData(spec, inputData)
		if err != nil {
			return canoto.Any{}, fmt.Errorf("failed to fill from kvData: %w", err)
		}
	}

	return kvPairs, nil
}

func unmarshalOutput(spec *canoto.Spec, b []byte) (canoto.Any, error) {
	a, err := canoto.Unmarshal(spec, b)
	if err != nil {
		return canoto.Any{}, fmt.Errorf("failed to unmarshal output: %w", err)
	}
	return a, nil
}

func fillFromInputData(spec *canoto.Spec, kvData map[string]string) (canoto.Any, error) {
	// Require exact match in required fields to supplied arguments
	if len(kvData) != len(spec.Fields) {
		return canoto.Any{}, fmt.Errorf("type has %d fields, got %d arguments", len(spec.Fields), len(kvData))
	}
	for i := range spec.Fields {
		if _, ok := kvData[spec.Fields[i].Name]; !ok {
			return canoto.Any{}, fmt.Errorf("missing argument: %s", spec.Fields[i].Name)
		}
	}

	a := canoto.Any{Fields: make([]canoto.AnyField, 0)}
	for i := range spec.Fields {
		value := kvData[spec.Fields[i].Name]
		switch spec.Fields[i].CachedWhichOneOfType() {
		case 6: // Int
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return canoto.Any{}, fmt.Errorf("failed to parse int: %w", err)
			}
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: v,
			})
		case 7: // Uint
			v, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return canoto.Any{}, fmt.Errorf("failed to parse uint: %w", err)
			}
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: v,
			})
		case 10: // Bool
			v, err := strconv.ParseBool(value)
			if err != nil {
				return canoto.Any{}, fmt.Errorf("failed to parse bool: %w", err)
			}
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: v,
			})
		case 11: // String
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: value,
			})
		case 12: // Bytes
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: []byte(value),
			})
		case 13: // Fixed Bytes
			// Special case: address
			if spec.Fields[i].TypeFixedBytes == codec.AddressLen {
				v, err := codec.StringToAddress(value)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to parse address: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: v[:],
				})
			} else {
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: []byte(value)[:], //nolint: gocritic
				})
			}
		default:
			return canoto.Any{}, fmt.Errorf("unsupported field type: %s", spec.Fields[i].Name)
		}
	}

	return a, nil
}

func askForFlags(spec *canoto.Spec) (canoto.Any, error) {
	a := canoto.Any{Fields: make([]canoto.AnyField, 0)}
	for i := range spec.Fields {
		switch spec.Fields[i].CachedWhichOneOfType() {
		case 6: // Int
			switch spec.Fields[i].TypeInt {
			case 1: // Int8
				v, err := prompt.Int(spec.Fields[i].Name, math.MaxInt8)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get int8: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: int64(v),
				})
			case 2: // Int16
				v, err := prompt.Int(spec.Fields[i].Name, math.MaxInt16)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get int16: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: int64(v),
				})
			case 3: // Int32
				v, err := prompt.Int(spec.Fields[i].Name, math.MaxInt32)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get int32: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: int64(v),
				})
			case 4: // Int64
				v, err := prompt.Int(spec.Fields[i].Name, math.MaxInt64)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get int64: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: int64(v),
				})
			}
		case 7: // Uint
			switch spec.Fields[i].TypeUint {
			case 1: // Uint8
				v, err := prompt.Uint(spec.Fields[i].Name, math.MaxUint8)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get uint8: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: uint64(v),
				})
			case 2: // Uint16
				v, err := prompt.Uint(spec.Fields[i].Name, math.MaxUint16)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get uint16: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: uint64(v),
				})
			case 3: // Uint32
				v, err := prompt.Uint(spec.Fields[i].Name, math.MaxUint32)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get uint32: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: uint64(v),
				})
			case 4: // Uint64
				v, err := prompt.Uint(spec.Fields[i].Name, math.MaxUint64)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get uint64: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: uint64(v),
				})
			}
		case 10: // Bool
			v, err := prompt.Bool(spec.Fields[i].Name)
			if err != nil {
				return canoto.Any{}, fmt.Errorf("failed to get bool: %w", err)
			}
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: v,
			})
		case 11: // String
			// TODO: fine tune max length
			v, err := prompt.String(spec.Fields[i].Name, 0, 1024)
			if err != nil {
				return canoto.Any{}, fmt.Errorf("failed to get string: %w", err)
			}
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: v,
			})
		case 12: // Bytes
			v, err := prompt.Bytes(spec.Fields[i].Name)
			if err != nil {
				return canoto.Any{}, fmt.Errorf("failed to get bytes: %w", err)
			}
			a.Fields = append(a.Fields, canoto.AnyField{
				Name:  spec.Fields[i].Name,
				Value: v,
			})
		case 13: // Fixed Bytes
			// Special case: address
			if spec.Fields[i].TypeFixedBytes == codec.AddressLen {
				v, err := prompt.Address(spec.Fields[i].Name)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get address: %w", err)
				}
				if len(v) != codec.AddressLen {
					return canoto.Any{}, fmt.Errorf("invalid address length: %d", len(v))
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: v[:],
				})
			} else {
				v, err := prompt.Bytes(spec.Fields[i].Name)
				if err != nil {
					return canoto.Any{}, fmt.Errorf("failed to get fixed bytes: %w", err)
				}
				a.Fields = append(a.Fields, canoto.AnyField{
					Name:  spec.Fields[i].Name,
					Value: v[:], //nolint: gocritic
				})
			}
		default:
			return canoto.Any{}, fmt.Errorf("unsupported field type: %s", spec.Fields[i].Name)
		}
	}
	return a, nil
}

func getType(fieldType *canoto.FieldType) string {
	switch fieldType.CachedWhichOneOfType() {
	case 6: // Int
		switch fieldType.TypeInt {
		case 1: // Int8
			return "int8"
		case 2: // Int16
			return "int16"
		case 3: // Int32
			return "int32"
		case 4: // Int64
			return "int64"
		}
	case 7: // Uint
		switch fieldType.TypeUint {
		case 1: // Uint8
			return "uint8"
		case 2: // Uint16
			return "uint16"
		case 3: // Uint32
			return "uint32"
		case 4: // Uint64
			return "uint64"
		}
	case 10: // Bool
		return "bool"
	case 11: // String
		return "string"
	case 12: // Bytes
		return "bytes"
	case 13: // Fixed Bytes
		// Special case: address
		if fieldType.TypeFixedBytes == codec.AddressLen {
			return "address"
		}
		var str strings.Builder
		str.WriteString("[")
		str.WriteString(strconv.FormatUint(fieldType.TypeFixedBytes, 10))
		str.WriteString("]byte")
		return str.String()
	}
	return "unknown type"
}
