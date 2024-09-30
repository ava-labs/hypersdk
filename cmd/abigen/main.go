// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/abi"
)

var (
	packageName string
	rootCmd     = &cobra.Command{
		Use:   "abigen <input_abi.json> <output_file.go>",
		Short: "Generate Go structs from ABI JSON",
		Args:  cobra.ExactArgs(2),
		RunE:  run,
	}
)

func init() {
	rootCmd.Flags().StringVarP(&packageName, "package", "p", "", "Package name for generated code (overrides default)")
}

func run(_ *cobra.Command, args []string) error {
	inputFile := args[0]
	outputFile := args[1]

	// Read the input ABI JSON file
	abiData, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("error reading input file: %w", err)
	}

	// Parse the ABI JSON
	var vmABI abi.ABI
	err = json.Unmarshal(abiData, &vmABI)
	if err != nil {
		return fmt.Errorf("error parsing ABI JSON: %w", err)
	}

	if packageName == "" {
		packageName = filepath.Base(filepath.Dir(outputFile))
	}

	// Generate Go structs
	generatedCode, err := abi.GenerateGoStructs(vmABI, packageName)
	if err != nil {
		return fmt.Errorf("error generating Go structs: %w", err)
	}

	// Create the directory for the output file if it doesn't exist
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Write the generated code to the output file
	err = os.WriteFile(outputFile, []byte(generatedCode), 0o600)
	if err != nil {
		return fmt.Errorf("error writing output file: %w", err)
	}

	fmt.Printf("Successfully generated Go structs in %s\n", outputFile)
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
