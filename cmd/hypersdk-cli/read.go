// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/StephenButtolph/canoto"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/auth"
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
		spec, ok := abi.FindActionSpecByName(actionName)
		if !ok {
			return fmt.Errorf("failed to find action spec: %s", actionName)
		}

		a, err := fillAction(cmd, spec)
		if err != nil {
			return fmt.Errorf("failed to fill action: %w", err)
		}

		actionBytes, err := canoto.Marshal(spec, a)
		if err != nil {
			return fmt.Errorf("failed to marshal action: %w", err)
		}

		typeID, ok := abi.GetActionID(actionName)
		if !ok {
			return fmt.Errorf("failed to get action ID: %s", actionName)
		}

		actionBytes = append([]byte{typeID}, actionBytes...)

		results, err := client.ExecuteActions(context.Background(), sender, [][]byte{actionBytes})
		if err != nil {
			return fmt.Errorf("failed to execute action: %w", err)
		}

		b := results[0]
		outputTypeID := b[0]
		outputSpec, ok := abi.FindOutputSpecByID(outputTypeID)
		if !ok {
			return fmt.Errorf("failed to find output spec: %d", outputTypeID)
		}

		output, err := unmarshalOutput(outputSpec, b[1:])
		if err != nil {
			return fmt.Errorf("failed to unmarshal output: %w", err)
		}

		return printValue(cmd, readResponse{Result: output})
	},
}

type readResponse struct {
	Result canoto.Any `json:"result"`
}

func (r readResponse) String() string {
	var result strings.Builder
	result.WriteString("✅ Read-only execution successful:\n")
	jsonValue, err := json.Marshal(r.Result)
	if err != nil {
		jsonValue = []byte(fmt.Sprintf("%v", r.Result))
	}
	result.WriteString(fmt.Sprintf("Result: %s\n", string(jsonValue)))
	return result.String()
}

func init() {
	readCmd.Flags().String("sender", "", "Address of the sender in hex")
	readCmd.Flags().StringToString("data", nil, "Key-value pairs for the action data (e.g., key1=value1,key2=value2)")
	rootCmd.AddCommand(readCmd)
}
