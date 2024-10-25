// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/abi/dynamic"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/codec"
)

var readCmd = &cobra.Command{
	Use:   "read [action]",
	Short: "Read data from the chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. figure out sender address
		senderStr, err := cmd.Flags().GetString("sender")
		if err != nil {
			return fmt.Errorf("failed to get sender: %w", err)
		}

		var sender codec.Address

		if senderStr != "" {
			sender, err = codec.StringToAddress(senderStr)
			if err != nil {
				return fmt.Errorf("failed to convert sender to address: %w", err)
			}
		} else {
			// ok, infer user's address from the private key
			keyString, err := getConfigValue(cmd, "key", true)
			if err != nil {
				return fmt.Errorf("failed to get key from config: %w", err)
			}
			key, err := privateKeyFromString(keyString)
			if err != nil {
				return fmt.Errorf("failed to decode key: %w", err)
			}
			sender = auth.NewED25519Address(key.PublicKey())
		}

		// 2. create client
		endpoint, err := getConfigValue(cmd, "endpoint", true)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		// 3. get abi
		abi, err := client.GetABI(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get abi: %w", err)
		}

		// 4. get action name from args
		if len(args) == 0 {
			return errors.New("action name is required")
		}
		actionName := args[0]
		_, found := abi.FindActionByName(actionName)
		if !found {
			return fmt.Errorf("failed to find action: %s", actionName)
		}

		typ, found := abi.FindTypeByName(actionName)
		if !found {
			return fmt.Errorf("failed to find type: %s", actionName)
		}

		// 5. create action using kvPairs
		kvPairs, err := fillAction(cmd, typ)
		if err != nil {
			return err
		}

		jsonPayload, err := json.Marshal(kvPairs)
		if err != nil {
			return fmt.Errorf("failed to marshal kvPairs: %w", err)
		}

		actionBytes, err := dynamic.Marshal(abi, actionName, string(jsonPayload))
		if err != nil {
			return fmt.Errorf("failed to marshal action: %w", err)
		}

		results, executeErr := client.ExecuteActions(context.Background(), sender, [][]byte{actionBytes})
		var resultStruct map[string]interface{}

		if len(results) == 1 {
			resultJSON, err := dynamic.UnmarshalOutput(abi, results[0])
			if err != nil {
				return fmt.Errorf("failed to unmarshal result: %w", err)
			}

			err = json.Unmarshal([]byte(resultJSON), &resultStruct)
			if err != nil {
				return fmt.Errorf("failed to unmarshal result JSON: %w", err)
			}
		}

		errorString := ""
		if executeErr != nil {
			errorString = executeErr.Error()
		}

		return printValue(cmd, readResponse{
			Result:  resultStruct,
			Success: executeErr == nil,
			Error:   errorString,
		})
	},
}

func fillAction(cmd *cobra.Command, typ abi.Type) (map[string]interface{}, error) {
	// get key-value pairs
	inputData, err := cmd.Flags().GetStringToString("data")
	if err != nil {
		return nil, fmt.Errorf("failed to get data key-value pairs: %w", err)
	}

	isJSONOutput, err := isJSONOutputRequested(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get output format: %w", err)
	}

	isInteractive := len(inputData) == 0 && !isJSONOutput

	var kvPairs map[string]interface{}
	if isInteractive {
		kvPairs, err = askForFlags(typ)
		if err != nil {
			return nil, fmt.Errorf("failed to ask for flags: %w", err)
		}
	} else {
		kvPairs, err = fillFromInputData(typ, inputData)
		if err != nil {
			return nil, fmt.Errorf("failed to fill from kvData: %w", err)
		}
	}

	return kvPairs, nil
}

func fillFromInputData(typ abi.Type, kvData map[string]string) (map[string]interface{}, error) {
	// Require exact match in required fields to supplied arguments
	if len(kvData) != len(typ.Fields) {
		return nil, fmt.Errorf("type has %d fields, got %d arguments", len(typ.Fields), len(kvData))
	}
	for _, field := range typ.Fields {
		if _, ok := kvData[field.Name]; !ok {
			return nil, fmt.Errorf("missing argument: %s", field.Name)
		}
	}

	kvPairs := make(map[string]interface{})
	for _, field := range typ.Fields {
		value := kvData[field.Name]
		var parsedValue interface{}
		var err error
		switch field.Type {
		case "Address":
			parsedValue = value
		case "uint8", "uint16", "uint32", "uint", "uint64":
			parsedValue, err = strconv.ParseUint(value, 10, 64)
		case "int8", "int16", "int32", "int", "int64":
			parsedValue, err = strconv.ParseInt(value, 10, 64)
		case "[]uint8":
			if value == "" {
				parsedValue = []uint8{}
			} else {
				parsedValue, err = codec.LoadHex(value, -1)
			}
		case "string":
			parsedValue = value
		case "bool":
			parsedValue, err = strconv.ParseBool(value)
		default:
			return nil, fmt.Errorf("unsupported field type: %s", field.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", field.Name, err)
		}
		kvPairs[field.Name] = parsedValue
	}
	return kvPairs, nil
}

func askForFlags(typ abi.Type) (map[string]interface{}, error) {
	kvPairs := make(map[string]interface{})

	for _, field := range typ.Fields {
		var err error
		var value interface{}
		switch field.Type {
		case "Address":
			value, err = prompt.Address(field.Name)
		case "uint8":
			value, err = prompt.Uint(field.Name, math.MaxUint8)
		case "uint16":
			value, err = prompt.Uint(field.Name, math.MaxUint16)
		case "uint32":
			value, err = prompt.Uint(field.Name, math.MaxUint32)
		case "uint", "uint64":
			value, err = prompt.Uint(field.Name, math.MaxUint64)
		case "int8":
			value, err = prompt.Int(field.Name, math.MaxInt8)
		case "int16":
			value, err = prompt.Int(field.Name, math.MaxInt16)
		case "int32":
			value, err = prompt.Int(field.Name, math.MaxInt32)
		case "int", "int64":
			value, err = prompt.Int(field.Name, math.MaxInt64)
		case "[]uint8":
			value, err = prompt.Bytes(field.Name)
		case "string":
			value, err = prompt.String(field.Name, 0, 1024)
		case "bool":
			value, err = prompt.Bool(field.Name)
		default:
			return nil, fmt.Errorf("unsupported field type in CLI: %s", field.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get input for %s field: %w", field.Name, err)
		}
		kvPairs[field.Name] = value
	}
	return kvPairs, nil
}

type readResponse struct {
	Result  map[string]interface{} `json:"result"`
	Success bool                   `json:"success"`
	Error   string                 `json:"error"`
}

func (r readResponse) String() string {
	var result strings.Builder
	if r.Success {
		result.WriteString("✅ Read-only execution successful:\n")
		for key, value := range r.Result {
			jsonValue, err := json.Marshal(value)
			if err != nil {
				jsonValue = []byte(fmt.Sprintf("%v", value))
			}
			result.WriteString(fmt.Sprintf("%s: %s\n", key, string(jsonValue)))
		}
	} else {
		result.WriteString(fmt.Sprintf("❌ Read-only execution failed: %s\n", r.Error))
	}
	return result.String()
}

func init() {
	readCmd.Flags().String("sender", "", "Address of the sender in hex")
	readCmd.Flags().StringToString("data", nil, "Key-value pairs for the action data (e.g., key1=value1,key2=value2)")
	rootCmd.AddCommand(readCmd)
}
