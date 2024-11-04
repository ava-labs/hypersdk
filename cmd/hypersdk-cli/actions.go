// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
)

var actionsCmd = &cobra.Command{
	Use:   "actions",
	Short: "Print the list of actions available in the ABI",
	RunE: func(cmd *cobra.Command, _ []string) error {
		endpoint, err := getConfigValue(cmd, "endpoint", true)
		if err != nil {
			return fmt.Errorf("failed to get endpoint: %w", err)
		}
		client := jsonrpc.NewJSONRPCClient(endpoint)

		abi, err := client.GetABI(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get ABI: %w", err)
		}

		return printValue(cmd, abiWrapper{ABI: abi})
	},
}

type abiWrapper struct {
	ABI abi.ABI
}

func (a abiWrapper) String() string {
	result := ""
	for _, action := range a.ABI.Actions {
		result += fmt.Sprintf("---\n%s\n\n", action.Name)
		typ, found := a.ABI.FindTypeByName(action.Name)
		if !found {
			result += fmt.Sprintf("  Error: Type not found for action %s\n", action.Name)
			continue
		} else {
			result += "Inputs:\n"
			for _, field := range typ.Fields {
				result += fmt.Sprintf("  %s: %s\n", field.Name, field.Type)
			}
		}

		output, found := a.ABI.FindOutputByID(action.ID)
		if !found {
			result += fmt.Sprintf("No outputs for %s with id %d\n", action.Name, action.ID)
			continue
		}

		typ, found = a.ABI.FindTypeByName(output.Name)
		if !found {
			result += fmt.Sprintf("  Error: Type not found for output %s\n", output.Name)
			continue
		}
		result += "\nOutputs:\n"
		for _, field := range typ.Fields {
			result += fmt.Sprintf("  %s: %s\n", field.Name, field.Type)
		}
	}
	return result
}

func init() {
	rootCmd.AddCommand(actionsCmd)
}
