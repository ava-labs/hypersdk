package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/abi/dynamic"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/cli/prompt"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
	Use:   "read [action]",
	Short: "Read data from the chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		//1. figure out sender address
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
			//ok, infer user's address from the private key
			keyString, err := getConfigValue(cmd, "key")
			if err != nil {
				return fmt.Errorf("failed to get key from config: %w", err)
			}
			key, err := privateKeyFromString(keyString)
			if err != nil {
				return fmt.Errorf("failed to decode key: %w", err)
			}
			sender = auth.NewED25519Address(key.PublicKey())
		}

		//2. create client
		endpoint, err := getConfigValue(cmd, "endpoint")
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		//3. get abi
		abi, err := client.GetABI(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get abi: %w", err)
		}

		//4. get action name from args
		if len(args) == 0 {
			return fmt.Errorf("action name is required")
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

		//4. get key-value pairs
		inputData, err := cmd.Flags().GetStringToString("data")
		if err != nil {
			return fmt.Errorf("failed to get data key-value pairs: %w", err)
		}

		shouldAskForFlags := len(inputData) == 0

		var kvPairs map[string]interface{}
		if shouldAskForFlags {
			kvPairs, err = askForFlags(typ)
			if err != nil {
				return fmt.Errorf("failed to ask for flags: %w", err)
			}
		} else {
			kvPairs, err = fillFromInputData(typ, inputData)
			if err != nil {
				return fmt.Errorf("failed to fill from kvData: %w", err)
			}
		}

		//5. create action using kvPairs
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

func fillFromInputData(typ abi.Type, kvData map[string]string) (map[string]interface{}, error) {
	kvPairs := make(map[string]interface{})
	for _, field := range typ.Fields {
		value, ok := kvData[field.Name]
		if !ok {
			continue
		}
		switch field.Type {
		case "Address":
			kvPairs[field.Name] = value
		case "uint64":
			parsedValue, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s as uint64: %w", field.Name, err)
			}
			kvPairs[field.Name] = parsedValue
		case "[]uint8":
			if value == "" {
				kvPairs[field.Name] = []uint8{}
			} else {
				decodedValue, err := base64.StdEncoding.DecodeString(value)
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 for %s: %w", field.Name, err)
				}
				kvPairs[field.Name] = decodedValue
			}
		default:
			return nil, fmt.Errorf("unsupported field type: %s", field.Type)
		}
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
		case "uint64":
			value, err = prompt.Amount(field.Name, ^uint64(0), nil)
		case "[]uint8":
			value, err = prompt.Bytes(field.Name)
		default:
			return nil, fmt.Errorf("unsupported field type: %s", field.Type)
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
		result.WriteString("✅ Result:\n")
		for key, value := range r.Result {
			jsonValue, err := json.Marshal(value)
			if err != nil {
				jsonValue = []byte(fmt.Sprintf("%v", value))
			}
			result.WriteString(fmt.Sprintf("%s: %s\n", key, string(jsonValue)))
		}
	} else {
		result.WriteString(fmt.Sprintf("❌ Error: %s\n", r.Error))
	}
	return result.String()
}

func init() {
	readCmd.Flags().String("sender", "", "Address of the sender in hex")
	readCmd.Flags().StringToString("data", nil, "Key-value pairs for the action data (e.g., key1=value1,key2=value2)")
	rootCmd.AddCommand(readCmd)
}
