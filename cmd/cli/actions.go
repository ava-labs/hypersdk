package main

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/spf13/cobra"
)

var actionsCmd = &cobra.Command{
	Use:   "actions",
	Short: "Print the list of actions available in the ABI",
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, err := getConfigValue(cmd, "endpoint")
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
		typ, found := findTypeByName(a.ABI.Types, action.Name)
		if !found {
			result += fmt.Sprintf("  Error: Type not found for action %s\n", action.Name)
			continue
		} else {
			result += "Inputs:\n"
			for _, field := range typ.Fields {
				result += fmt.Sprintf("  %s: %s\n", field.Name, field.Type)
			}
		}

		output, found := findOutputByID(a.ABI.Outputs, action.ID)
		if !found {
			result += fmt.Sprintf("No outputs for %s with id %d\n", action.Name, action.ID)
			continue
		}

		typ, found = findTypeByName(a.ABI.Types, output.Name)
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

func findOutputByID(outputs []abi.TypedStruct, id uint8) (abi.TypedStruct, bool) {
	for _, output := range outputs {
		if output.ID == id {
			return output, true
		}
	}
	return abi.TypedStruct{}, false
}

func findTypeByName(types []abi.Type, name string) (abi.Type, bool) {
	for _, typ := range types {
		if typ.Name == name {
			return typ, true
		}
	}
	return abi.Type{}, false
}

func init() {
	rootCmd.AddCommand(actionsCmd)
}
